// Local presentation state only; server owns authentication and generation rules.
document.addEventListener('alpine:init', () => {
 Alpine.data('shell', () => ({
  toggleTheme() {
   const dark = document.documentElement.classList.toggle('dark');
   try { localStorage.setItem('theme', dark ? 'dark' : 'light'); } catch {}
  }
 }));
});
document.addEventListener('htmx:sendError', () => {
 const target = document.getElementById('adminNotice') || document.getElementById('loginErrorAlert');
 if (target) target.textContent = 'Connection failed. Please try again.';
});

// Only recurring admin status reads pause; explicit form mutations are unaffected.
document.addEventListener('htmx:beforeRequest', event => {
 const path=event.detail.requestConfig?.path;
 if(document.hidden && ['/ui/admin/status','/ui/admin/snapshots'].includes(path)) event.preventDefault();
});
document.addEventListener('visibilitychange', () => {
 if(!document.hidden && document.getElementById('adminStats')) htmx.trigger(document.body,'admin-refresh');
});
