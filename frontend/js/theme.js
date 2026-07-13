/* Light/dark theme switch. Dark is the default.
   Loaded in <head> so the saved theme applies before first paint. */
(function () {
  var KEY = 'ui_theme';

  function isLight() {
    return document.documentElement.classList.contains('theme-light');
  }
  function apply(theme) {
    document.documentElement.classList.toggle('theme-light', theme === 'light');
  }

  // Apply saved theme immediately (default: dark)
  var saved = null;
  try { saved = localStorage.getItem(KEY); } catch (e) {}
  apply(saved === 'light' ? 'light' : 'dark');

  var SUN_SVG =
    '<svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true">' +
    '<circle cx="12" cy="12" r="4"/>' +
    '<path d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>';
  var MOON_SVG =
    '<svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' +
    '<path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/></svg>';

  function mountToggle() {
    var header = document.querySelector('header');
    if (!header || document.querySelector('.theme-toggle')) return;

    var btn = document.createElement('button');
    btn.className = 'theme-toggle';
    btn.type = 'button';

    function refresh() {
      // In dark mode show the sun (switch to light), and vice versa
      btn.innerHTML = isLight() ? MOON_SVG : SUN_SVG;
      btn.setAttribute('aria-label', isLight() ? 'Включить тёмную тему' : 'Включить светлую тему');
      btn.title = btn.getAttribute('aria-label');
    }
    refresh();

    btn.addEventListener('click', function () {
      var next = isLight() ? 'dark' : 'light';
      apply(next);
      try { localStorage.setItem(KEY, next); } catch (e) {}
      refresh();
    });

    var userInfo = header.querySelector('.user-info');
    if (userInfo) userInfo.appendChild(btn);
    else header.appendChild(btn);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', mountToggle);
  } else {
    mountToggle();
  }
})();
