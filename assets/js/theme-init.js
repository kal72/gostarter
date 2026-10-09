(function() {
      try {
        var p = new URLSearchParams(window.location.search);
        var t = p.get('theme');
        if (t === 'dark') {
          document.documentElement.classList.add('dark');
          localStorage.setItem('theme', 'dark');
          return;
        } else if (t === 'light') {
          document.documentElement.classList.remove('dark');
          localStorage.setItem('theme', 'light');
          return;
        }
        var s = localStorage.getItem('theme');
        if (s === 'dark') {
          document.documentElement.classList.add('dark');
        } else {
          document.documentElement.classList.remove('dark');
          if (!s) localStorage.setItem('theme', 'light');
        }
      } catch (e) {}
    })();
