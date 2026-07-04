// static/js/address-autocomplete.js
// Reusable address autocomplete component.
// Attach to any address input with data-address-autocomplete attribute.
// Related fields identified by data-address-city, data-address-state, etc.
(function() {
    'use strict';

    var debounceTimer;
    var activeDropdown = null;

    document.addEventListener('input', function(e) {
        var input = e.target;
        if (!input.hasAttribute('data-address-autocomplete')) return;

        clearTimeout(debounceTimer);
        var query = input.value.trim();

        if (query.length < 2) {
            closeDropdown();
            return;
        }

        debounceTimer = setTimeout(function() {
            fetch('/api/addresses?q=' + encodeURIComponent(query))
                .then(function(r) { return r.json(); })
                .then(function(results) {
                    if (!results || results.length === 0) {
                        closeDropdown();
                        return;
                    }
                    showDropdown(input, results);
                })
                .catch(function() { closeDropdown(); });
        }, 250);
    });

    function showDropdown(input, results) {
        closeDropdown();

        var container = input.parentElement;
        container.style.position = 'relative';

        var dropdown = document.createElement('div');
        dropdown.className = 'address-dropdown';
        dropdown.style.cssText = 'position:absolute; top:100%; left:0; right:0; z-index:100; ' +
            'background:#1e293b; border:1px solid rgba(255,255,255,0.15); border-radius:8px; ' +
            'margin-top:4px; overflow:hidden; box-shadow:0 8px 24px rgba(0,0,0,0.3);';

        results.forEach(function(addr) {
            var item = document.createElement('div');
            item.style.cssText = 'padding:10px 14px; cursor:pointer; font-size:13px; ' +
                'color:rgba(255,255,255,0.7); border-bottom:1px solid rgba(255,255,255,0.06); ' +
                'transition:background 0.1s;';

            var parts = [addr.address];
            if (addr.city) parts.push(addr.city);
            if (addr.state) parts.push(addr.state);
            if (addr.zip) parts.push(addr.zip);
            item.textContent = parts.join(', ');

            item.addEventListener('mouseenter', function() {
                item.style.background = 'rgba(255,255,255,0.05)';
                item.style.color = '#ffffff';
            });
            item.addEventListener('mouseleave', function() {
                item.style.background = 'transparent';
                item.style.color = 'rgba(255,255,255,0.7)';
            });

            item.addEventListener('click', function() {
                fillAddress(input, addr);
                closeDropdown();
            });

            dropdown.appendChild(item);
        });

        container.appendChild(dropdown);
        activeDropdown = dropdown;
    }

    function fillAddress(input, addr) {
        input.value = addr.address || '';

        var form = input.closest('form');
        if (!form) return;

        var cityField = form.querySelector('[data-address-city]');
        var stateField = form.querySelector('[data-address-state]');
        var zipField = form.querySelector('[data-address-zip]');
        var countryField = form.querySelector('[data-address-country]');

        if (cityField) cityField.value = addr.city || '';
        if (stateField) stateField.value = addr.state || '';
        if (zipField) zipField.value = addr.zip || '';
        if (countryField) countryField.value = addr.country || '';
    }

    function closeDropdown() {
        if (activeDropdown && activeDropdown.parentElement) {
            activeDropdown.parentElement.removeChild(activeDropdown);
        }
        activeDropdown = null;
    }

    // Close dropdown on click outside
    document.addEventListener('click', function(e) {
        if (activeDropdown && !e.target.hasAttribute('data-address-autocomplete')) {
            closeDropdown();
        }
    });

    // Close dropdown on Escape
    document.addEventListener('keydown', function(e) {
        if (e.key === 'Escape') closeDropdown();
    });
})();
