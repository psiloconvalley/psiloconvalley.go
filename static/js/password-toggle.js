// static/js/password-toggle.js
// Password visibility toggle — reusable across all auth pages.
// Injects a filled eye icon inside each password input.
// Styled to match the dark auth card aesthetic (Stripe/Notion style).
(function() {
    // Inject styles once
    var style = document.createElement('style');
    style.textContent = [
        '.pw-wrap {',
        '  position: relative;',
        '  display: block;',
        '}',
        '.pw-wrap input {',
        '  padding-right: 44px !important;',
        '}',
        '.pw-toggle {',
        '  position: absolute;',
        '  right: 12px;',
        '  top: 50%;',
        '  transform: translateY(-50%);',
        '  background: none;',
        '  border: none;',
        '  cursor: pointer;',
        '  padding: 4px;',
        '  display: flex;',
        '  align-items: center;',
        '  justify-content: center;',
        '  color: rgba(255,255,255,0.20);',
        '  transition: color 0.2s ease;',
        '  outline: none;',
        '  -webkit-tap-highlight-color: transparent;',
        '}',
        '.pw-toggle:hover {',
        '  color: rgba(255,255,255,0.50);',
        '}',
        '.pw-toggle:focus-visible {',
        '  color: rgba(255,255,255,0.50);',
        '}',
        '.pw-toggle svg {',
        '  pointer-events: none;',
        '}',
    ].join('\n');
    document.head.appendChild(style);

    // Eye open — filled style (password hidden — "click to reveal")
    // Soft filled eye shape with a solid pupil
    var eyeOpen = [
        '<svg width="20" height="20" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">',
        '  <path d="M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5z" fill="currentColor" opacity="0.15"/>',
        '  <path d="M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5z" fill="none" stroke="currentColor" stroke-width="1.5"/>',
        '  <circle cx="12" cy="12" r="3.5" fill="currentColor" opacity="0.7"/>',
        '</svg>',
    ].join('');

    // Eye closed — filled style with slash (password visible — "click to hide")
    var eyeClosed = [
        '<svg width="20" height="20" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">',
        '  <path d="M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5z" fill="currentColor" opacity="0.08"/>',
        '  <path d="M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5z" fill="none" stroke="currentColor" stroke-width="1.5"/>',
        '  <circle cx="12" cy="12" r="3.5" fill="currentColor" opacity="0.3"/>',
        '  <line x1="4" y1="4" x2="20" y2="20" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>',
        '</svg>',
    ].join('');

    document.querySelectorAll('input[type="password"]').forEach(function(input) {
        // Create wrapper
        var wrapper = document.createElement('div');
        wrapper.className = 'pw-wrap';

        // Insert wrapper before input, then move input inside
        input.parentNode.insertBefore(wrapper, input);
        wrapper.appendChild(input);

        // Create toggle button
        var btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'pw-toggle';
        btn.setAttribute('aria-label', 'Toggle password visibility');
        btn.innerHTML = eyeOpen;
        wrapper.appendChild(btn);

        btn.addEventListener('click', function() {
            if (input.type === 'password') {
                input.type = 'text';
                btn.innerHTML = eyeClosed;
            } else {
                input.type = 'password';
                btn.innerHTML = eyeOpen;
            }
        });
    });
})();
