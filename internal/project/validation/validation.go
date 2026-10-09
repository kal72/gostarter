// Package validation defines the application-owned build matrix and mandatory smoke fixture.
package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"gostarter/internal/project/plan"
	"gostarter/internal/project/render"
	"gostarter/internal/project/templates"
	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"
)

const Version = "2"

func Key() string {
	s := sha256.Sum256([]byte("validation/" + Version + "/" + plan.EngineVersion + "/" + templates.Version + "/" + templates.Digest() + "/" + smokeFixture + "/" + runtime.GOOS + "/" + runtime.GOARCH + "/" + runtime.Version()))
	return hex.EncodeToString(s[:])
}

type Result struct{ ContentSHA256, IdentitySHA256 string }
type Engine struct {
	WorkspaceDir string
	MaxCases     int
	CaseCache    CaseCache
}

func (e Engine) Plan(ctx context.Context, p *plan.Plan, files profile.Source) (Result, error) {
	a, err := render.MaterializeAt(ctx, p, files, e.WorkspaceDir)
	if err != nil {
		return Result{}, err
	}
	if err = a.VerifyFor(p); err != nil {
		return Result{}, err
	}
	if err = e.smoke(ctx, p, a); err != nil {
		return Result{}, err
	}
	return Result{a.Receipt.ContentSHA256, a.Receipt.IdentitySHA256}, nil
}
func Matrix(files profile.Source, id plan.Snapshot, max int) ([]*plan.Plan, error) {
	catalog, err := manifest.Parse(files[manifest.FileName])
	if err != nil {
		return nil, err
	}
	selections := [][]string{{}}
	all := []string{}
	for _, ext := range catalog.Extensions {
		selections = append(selections, []string{ext.ID})
		all = append(all, ext.ID)
	}
	sort.Strings(all)
	selections = append(selections, all)
	var result []*plan.Plan
	seen := map[string]bool{}
	for _, protocol := range []string{"dual", "grpc"} {
		for _, selection := range selections {
			p, err := plan.Build(files, id, plan.Config{Module: "example.com/validated", ServiceName: "validated", Protocol: protocol, RequestedExtensions: selection})
			if err != nil {
				return nil, err
			}
			if !seen[p.Fingerprint] {
				result = append(result, p)
				seen[p.Fingerprint] = true
			}
		}
	}
	if len(result) > max {
		return nil, fmt.Errorf("candidate matrix requires %d cases; budget %d", len(result), max)
	}
	return result, nil
}
func (e Engine) Snapshot(ctx context.Context, id plan.Snapshot, files profile.Source) (Result, error) {
	cases, err := Matrix(files, id, e.MaxCases)
	if err != nil {
		return Result{}, err
	}
	execution := "uncached"
	if e.CaseCache != nil {
		execution, err = render.ValidationExecutionFingerprint(ctx, e.WorkspaceDir, cases[0])
		if err != nil {
			return Result{}, err
		}
	}
	run := e.Plan
	if e.CaseCache != nil {
		run = func(ctx context.Context, p *plan.Plan, files profile.Source) (Result, error) {
			result, err := e.Plan(ctx, p, files)
			if err != nil {
				return Result{}, err
			}
			current, err := render.ValidationExecutionFingerprint(ctx, e.WorkspaceDir, p)
			if err != nil {
				return Result{}, err
			}
			if current != execution {
				return Result{}, fmt.Errorf("validation execution environment changed")
			}
			return result, nil
		}
	}
	result, err := validateCases(ctx, id, files, cases, execution, e.CaseCache, run)
	if err != nil {
		return Result{}, err
	}
	if e.CaseCache != nil {
		current, err := render.ValidationExecutionFingerprint(ctx, e.WorkspaceDir, cases[0])
		if err != nil {
			return Result{}, err
		}
		if current != execution {
			return Result{}, fmt.Errorf("validation execution environment changed")
		}
	}
	return result, nil
}

func (e Engine) smoke(ctx context.Context, p *plan.Plan, a *render.Artifact) (err error) {
	stage, err := os.MkdirTemp(e.WorkspaceDir, "gostarter-smoke-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(stage)) }()
	dir := filepath.Join(stage, "project")
	if err = render.WriteNew(dir, a.Files); err != nil {
		return err
	}
	imports := ""
	args := "healthhandler.NewHandler(probe{})"
	features := 0
	user := false
	for _, ext := range p.Resolved.Extensions {
		if ext.Kind != "feature" {
			continue
		}
		alias := "feature" + strconv.Itoa(features)
		imports += "\n" + alias + " " + strconv.Quote(p.Config.Module+"/"+ext.Path+"/handler")
		args += ", &" + alias + ".Handler{}"
		features++
		user = user || ext.Path == "internal/user"
	}
	httpArg := ""
	gatewayInit := ""
	if p.Config.Protocol == "dual" {
		httpArg = "gateway,"
		imports += "\n\"net/http/httptest\""
		gatewayInit = `gateway,closeGateway,err:=NewHTTPGateway(ctx,cfg,services);if err!=nil{t.Fatal(err)};defer closeGateway()
 recorder:=httptest.NewRecorder();gateway.Handler.ServeHTTP(recorder,httptest.NewRequest("OPTIONS","/healthz",nil));if recorder.Code!=204{t.Fatal("gateway CORS",recorder.Code)}`
	}
	body := strings.NewReplacer("MODULE", p.Config.Module, "IMPORTS", imports, "ARGS", args, "COUNT", strconv.Itoa(features+1), "HASUSER", strconv.FormatBool(user), "HTTPARG", httpArg, "GATEWAYINIT", gatewayInit).Replace(smokeFixture)
	if err = os.WriteFile(filepath.Join(dir, "bootstrap/gostarter_validation_test.go"), []byte(body), 0600); err != nil {
		return err
	}
	return render.RunSmoke(ctx, dir, p.Config.GoVersion)
}

const smokeFixture = `package bootstrap
import (
 "context"
 "net"
 "testing"
 "time"
 "MODULE/infrastructure/config"
 "MODULE/pkg/logger"
 healthhandler "MODULE/internal/health/handler"
 pb "MODULE/gen/pb/health"
 "google.golang.org/grpc"
 "google.golang.org/grpc/codes"
 "google.golang.org/grpc/status"
 "google.golang.org/grpc/credentials/insecure"
 "google.golang.org/grpc/test/bufconn"
 "google.golang.org/protobuf/types/known/emptypb"
 IMPORTS
)
type probe struct{}
func(probe)Ping(context.Context)error{return nil}
func TestGoStarterValidation(t *testing.T){
 cfg:=&config.Config{App:config.AppConfig{Environment:"production"},Server:config.ServerConfig{DefaultTimeout:time.Second,ShutdownTimeout:time.Second},JWT:config.JWTConfig{SecretKey:"test-secret"}}
 log:=logger.NewNop();services:=ProvideServices(ARGS);server,cleanup:=NewGRPCServer(cfg,log,services);defer cleanup()
 if len(server.GetServiceInfo())!=COUNT{t.Fatal(server.GetServiceInfo())}
 listener:=bufconn.Listen(1024*1024);defer listener.Close();go server.Serve(listener)
 conn,err:=grpc.NewClient("passthrough:///bufnet",grpc.WithTransportCredentials(insecure.NewCredentials()),grpc.WithContextDialer(func(context.Context,string)(net.Conn,error){return listener.Dial()}));if err!=nil{t.Fatal(err)};defer conn.Close()
 ctx,cancel:=context.WithTimeout(context.Background(),3*time.Second);defer cancel()
 client:=pb.NewHealthServiceClient(conn)
 if r,err:=client.Check(ctx,&pb.HealthCheckRequest{});err!=nil||r.Status!=pb.HealthCheckResponse_STATUS_SERVING{t.Fatalf("health: %v %v",r,err)}
 if r,err:=client.Ready(ctx,&pb.HealthCheckRequest{});err!=nil||r.Status!=pb.HealthCheckResponse_STATUS_SERVING{t.Fatalf("ready: %v %v",r,err)}
 if HASUSER{err:=conn.Invoke(ctx,"/user.v1.UserService/DeleteUser",&emptypb.Empty{},&emptypb.Empty{});if status.Code(err)!=codes.Unauthenticated{t.Fatalf("JWT: %v",err)}}
 GATEWAYINIT
 resources:=&Resources{Logger:log};app,closeApp:=NewApp(cfg,log,nil,server,HTTPARG nil,nil,resources);app.Shutdown(ctx);closeApp();resources.Close(ctx)
}
`
