// static/js/address-autocomplete.js
// Reusable address autocomplete component.
// Supports local (instant fill) and Google (two-step: suggest then details).
//
// Usage: add data attributes to address inputs:
//   data-address-autocomplete  — the street address trigger field
//   data-address-city          — city fill target
//   data-address-state         — state fill target (input or select)
//   data-address-zip           — zip fill target
//   data-address-country       — country fill target
(function () {
    'use strict';

    var debounceTimer;
    var activeDropdown = null;
    var activeInput = null;

    // ── Listen for input on any address field ─────────────────────────────
    document.addEventListener('input', function (e) {
        var input = e.target;
        if (!input.hasAttribute('data-address-autocomplete')) return;

        clearTimeout(debounceTimer);
        var query = input.value.trim();

        if (query.length < 2) {
            closeDropdown();
            return;
        }

        activeInput = input;
        debounceTimer = setTimeout(function () {
            fetch('/api/addresses?q=' + encodeURIComponent(query))
                .then(function (r) { return r.json(); })
                .then(function (results) {
                    if (!results || results.length === 0) {
                        closeDropdown();
                        return;
                    }
                    showDropdown(input, results);
                })
                .catch(function () { closeDropdown(); });
        }, 300);
    });

    // ── Render dropdown ───────────────────────────────────────────────────
    function showDropdown(input, results) {
        closeDropdown();

        var container = input.parentElement;
        container.style.position = 'relative';

        var dropdown = document.createElement('div');
        dropdown.className = 'address-dropdown';
        dropdown.style.cssText = [
            'position:absolute',
            'top:100%',
            'left:0',
            'right:0',
            'z-index:200',
            'background:#1e293b',
            'border:1px solid rgba(255,255,255,0.15)',
            'border-radius:8px',
            'margin-top:4px',
            'overflow:hidden',
            'box-shadow:0 8px 24px rgba(0,0,0,0.4)'
        ].join(';');

        results.forEach(function (suggestion) {
            var item = document.createElement('div');
            item.style.cssText = [
                'padding:10px 14px',
                'cursor:pointer',
                'font-size:13px',
                'color:rgba(255,255,255,0.75)',
                'border-bottom:1px solid rgba(255,255,255,0.06)',
                'transition:background 0.1s',
                'display:flex',
                'align-items:center',
                'gap:8px'
            ].join(';');

            // Source indicator dot
            var dot = document.createElement('span');
            dot.style.cssText = [
                'width:6px',
                'height:6px',
                'border-radius:50%',
                'flex-shrink:0',
                'background:' + (suggestion.source === 'google'
                    ? '#4ade80'
                    : 'rgba(255,255,255,0.3)')
            ].join(';');

            var text = document.createElement('span');
            text.textContent = suggestion.label || suggestion.address || '';

            item.appendChild(dot);
            item.appendChild(text);

            item.addEventListener('mouseenter', function () {
                item.style.background = 'rgba(255,255,255,0.06)';
                item.style.color = '#ffffff';
            });
            item.addEventListener('mouseleave', function () {
                item.style.background = 'transparent';
                item.style.color = 'rgba(255,255,255,0.75)';
            });

            item.addEventListener('click', function () {
                closeDropdown();
                if (suggestion.needs_details && suggestion.place_id) {
                    // Google suggestion — fetch full details first
                    fetchPlaceDetails(input, suggestion.place_id);
                } else {
                    // Local suggestion — fill immediately
                    fillAddress(input, suggestion);
                }
            });

            dropdown.appendChild(item);
        });

        container.appendChild(dropdown);
        activeDropdown = dropdown;
    }

    // ── Fetch Google place details then fill ──────────────────────────────
    function fetchPlaceDetails(input, placeID) {
        fetch('/api/addresses/place?id=' + encodeURIComponent(placeID))
            .then(function (r) { return r.json(); })
            .then(function (details) {
                if (details) fillAddress(input, details);
            })
            .catch(function () {
                // fail silently — user can type the rest manually
            });
    }

    // ── Fill form fields ──────────────────────────────────────────────────
    function fillAddress(input, data) {
        // Fill the trigger field
        if (data.address) input.value = data.address;

        var form = input.closest('form');
        if (!form) return;

        // Fill city
        var cityField = form.querySelector('[data-address-city]');
        if (cityField && data.city) cityField.value = data.city;

        // Fill zip
        var zipField = form.querySelector('[data-address-zip]');
        if (zipField && data.zip) zipField.value = data.zip;

        // Fill country
        var countryField = form.querySelector('[data-address-country]');
        if (countryField && data.country) countryField.value = data.country;

        // Fill state — handles both <input> and <select>
        var stateField = form.querySelector('[data-address-state]');
        if (stateField && data.state) {
            if (stateField.tagName === 'SELECT') {
                setSelectValue(stateField, data.state);
            } else {
                stateField.value = data.state;
            }
        }
    }

    // ── Set <select> value by matching option text or value ───────────────
    function setSelectValue(select, value) {
        var options = select.options;
        var lower = value.toLowerCase();

        // Try exact match first
        for (var i = 0; i < options.length; i++) {
            if (options[i].value.toLowerCase() === lower ||
                options[i].text.toLowerCase() === lower) {
                select.selectedIndex = i;
                return;
            }
        }

        // Try partial match (e.g. "CA" matches "California")
        for (var j = 0; j < options.length; j++) {
            if (options[j].value.toLowerCase().indexOf(lower) === 0 ||
                options[j].text.toLowerCase().indexOf(lower) === 0) {
                select.selectedIndex = j;
                return;
            }
        }
    }

    // ── Close dropdown ────────────────────────────────────────────────────
    function closeDropdown() {
        if (activeDropdown && activeDropdown.parentElement) {
            activeDropdown.parentElement.removeChild(activeDropdown);
        }
        activeDropdown = null;
    }

    // Close on outside click
    document.addEventListener('click', function (e) {
        if (!activeDropdown) return;
        if (e.target.hasAttribute('data-address-autocomplete')) return;
        if (activeDropdown.contains(e.target)) return;
        closeDropdown();
    });

    // Close on Escape
    document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') closeDropdown();
    });
})();
