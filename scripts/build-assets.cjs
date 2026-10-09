const fs = require('node:fs');
const {execFileSync} = require('node:child_process');
fs.mkdirSync('assets/vendor', {recursive: true});
fs.copyFileSync('node_modules/htmx.org/dist/htmx.min.js', 'assets/vendor/htmx.min.js');
fs.copyFileSync('node_modules/htmx.org/LICENSE', 'assets/vendor/htmx.LICENSE');
fs.copyFileSync('node_modules/alpinejs/dist/cdn.min.js', 'assets/vendor/alpine.min.js');
execFileSync(process.execPath, ['node_modules/tailwindcss/lib/cli.js', '-i', 'assets/css/input.css', '-o', 'assets/css/tailwind.css', '--minify'], {stdio:'inherit'});
