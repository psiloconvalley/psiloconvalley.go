/**
 * PsiloconValley Invoice UI Controller
 * Architecture: Event Delegation, Integer Math (Cents), Zero-Trust Client State
 */
(function () {
    'use strict';

    // =========================================================================
    // 1. CONFIGURATION & STATE
    // =========================================================================
    const CURRENCY_SYMBOLS = {
        USD: '$', CAD: 'CA$', GBP: '£', EUR: '€'
    };

    // =========================================================================
    // 2. UTILITY FUNCTIONS (MATH & FORMATTING)
    // =========================================================================

    /**
     * Safely parses a string input into integer CENTS to avoid IEEE 754 float errors.
     * e.g., "10.50" -> 1050, "10" -> 1000, "" -> 0
     */
    function parseToCents(val) {
        if (!val) return 0;
        // Strip everything except numbers, dots, and minus signs
        const cleaned = String(val).replace(/[^0-9.\-]/g, '');
        const floatVal = parseFloat(cleaned);
        if (isNaN(floatVal)) return 0;
        return Math.round(floatVal * 100);
    }

    /**
     * Formats integer cents back to a localized currency string.
     */
    function formatCents(cents) {
        const symbol = getCurrencySymbol();
        // Handle negative cents just in case, though UI should prevent it
        const isNegative = cents < 0;
        const absCents = Math.abs(cents);
        const dollars = (absCents / 100).toFixed(2);
        return `${isNegative ? '-' : ''}${symbol}${dollars}`;
    }

    function getCurrencySymbol() {
        const sel = document.getElementById('currency');
        const code = sel ? sel.value : 'USD';
        return CURRENCY_SYMBOLS[code] || '$';
    }

    // =========================================================================
    // 3. CORE CALCULATION ENGINE
    // =========================================================================

    /**
     * Recalculates a single row and returns its total in cents.
     */
    function calculateRow(tr) {
        const qtyInput = tr.querySelector('.qty');
        const priceInput = tr.querySelector('.price');
        const amountCell = tr.querySelector('.row-amount');

        const qty = parseFloat(qtyInput?.value || 0) || 0;
        const priceCents = parseToCents(priceInput?.value);
        
        // Math in integers!
        const lineCents = Math.round(qty * priceCents);

        if (amountCell) {
            amountCell.textContent = formatCents(lineCents);
        }

        return lineCents;
    }

    /**
     * Recalculates the entire invoice (Subtotal, Tax, Total).
     */
    function recalculateInvoice() {
        const tbody = document.getElementById('items_body');
        if (!tbody) return;

        const rows = tbody.querySelectorAll('tr');
        let subtotalCents = 0;

        rows.forEach(tr => {
            subtotalCents += calculateRow(tr);
        });

        // Tax Calculation (Matching Go backend logic: bps / 10000)
        const taxRateInput = document.getElementById('tax_rate');
        const taxRatePercent = parseFloat(taxRateInput?.value || 0) || 0;
        const taxCents = Math.round((subtotalCents * taxRatePercent) / 100);

        const totalCents = subtotalCents + taxCents;

        // Update DOM
        const elSubtotal = document.getElementById('subtotal');
        const elTax = document.getElementById('tax_amount');
        const elTotal = document.getElementById('total');

        if (elSubtotal) elSubtotal.textContent = formatCents(subtotalCents);
        if (elTax) elTax.textContent = formatCents(taxCents);
        if (elTotal) elTotal.textContent = formatCents(totalCents);
    }

    // =========================================================================
    // 4. DOM MANIPULATION (ADD / REMOVE)
    // =========================================================================

    function addRow() {
        const tbody = document.getElementById('items_body');
        if (!tbody) return;

        const rows = tbody.querySelectorAll('tr');
        if (rows.length === 0) return; // Safety check

        // Clone the last row
        const lastRow = rows[rows.length - 1];
        const newRow = lastRow.cloneNode(true);

        // Reset values for the new row
        const desc = newRow.querySelector('.item-name');
        if (desc) desc.value = '';

        const qty = newRow.querySelector('.qty');
        if (qty) qty.value = '1';

        const price = newRow.querySelector('.price');
        if (price) price.value = '0.00';

        const amount = newRow.querySelector('.row-amount');
        if (amount) amount.textContent = formatCents(0);

        tbody.appendChild(newRow);
        
        // Focus the new description input for fast typing
        if (desc) desc.focus();
        
        recalculateInvoice();
    }

    function removeRow(btn) {
        const tbody = document.getElementById('items_body');
        if (!tbody) return;

        const rows = tbody.querySelectorAll('tr');
        const tr = btn.closest('tr');

        if (rows.length <= 1) {
            // Don't delete the last row, just clear it
            const desc = tr.querySelector('.item-name');
            if (desc) desc.value = '';
            const qty = tr.querySelector('.qty');
            if (qty) qty.value = '1';
            const price = tr.querySelector('.price');
            if (price) price.value = '0.00';
        } else {
            tr.remove();
        }

        recalculateInvoice();
    }

    // =========================================================================
    // 5. EVENT DELEGATION & INITIALIZATION
    // =========================================================================

    function init() {
        const tbody = document.getElementById('items_body');
        
        // EVENT DELEGATION: One listener to rule them all.
        // This catches events on dynamically added rows automatically.
        if (tbody) {
            tbody.addEventListener('input', function (e) {
                const target = e.target;
                if (target.classList.contains('qty') || target.classList.contains('price')) {
                    recalculateInvoice();
                }
            });
        }

        // Sidebar listeners
        const taxRate = document.getElementById('tax_rate');
        if (taxRate) taxRate.addEventListener('input', recalculateInvoice);

        const currency = document.getElementById('currency');
        if (currency) currency.addEventListener('change', recalculateInvoice);

        // Expose ONLY what the HTML template inline handlers require
        window.addRow = addRow;
        window.removeRow = removeRow;

        // Initial calculation on page load
        recalculateInvoice();
    }

    // Boot up when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

})();
