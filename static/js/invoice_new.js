// ── Currency symbol helper ──────────────────────────────────────────
function getCurrencySymbol() {
    const sel = document.getElementById('currency');
    if (!sel || sel.selectedIndex < 0) return '$';
    return sel.options[sel.selectedIndex].dataset.symbol || '$';
}

// ── Live totals calculator ─────────────────────────────────────────
// Runs on every qty/price/tax/currency change.
// Updates both the sidebar display and the mobile bottom bar.
// Also updates each row's amount cell / card total.
function calculateLiveTotals() {
    const sym = getCurrencySymbol();
    let subtotal = 0;
    const isMobile = window.innerWidth <= MOBILE_BREAKPOINT;

    if (isMobile) {
        // Read from mobile cards — these are the active inputs on mobile
        document.querySelectorAll('#items_body_mobile .item-card-mobile').forEach(card => {
            const qty   = parseFloat(card.querySelector('.qty')?.value)   || 0;
            const price = parseFloat(card.querySelector('.price')?.value) || 0;
            const amt   = qty * price;
            subtotal += amt;
            const totalEl = card.querySelector('.row-total');
            if (totalEl) totalEl.textContent = sym + amt.toFixed(2);
        });
    } else {
        // Read from desktop rows — these are the active inputs on desktop
        document.querySelectorAll('#items_body_desktop tr').forEach(row => {
            const qty   = parseFloat(row.querySelector('.qty')?.value)   || 0;
            const price = parseFloat(row.querySelector('.price')?.value) || 0;
            const amt   = qty * price;
            subtotal += amt;
            const amtEl = row.querySelector('.row-amount');
            if (amtEl) amtEl.textContent = sym + amt.toFixed(2);
        });
    }

    const taxRate      = parseFloat(document.getElementById('tax_rate')?.value) || 0;
    const taxAmt       = subtotal * (taxRate / 100);
    const discountAmt  = parseFloat(document.getElementById('discount_amount')?.value) || 0;
    const total        = subtotal + taxAmt - discountAmt;

    const fmt = v => sym + v.toFixed(2);
    const el  = id => document.getElementById(id);

    // Show/hide discount line in summary
    const discountLine = document.getElementById('discount_line');
    if (discountLine) discountLine.style.display = discountAmt > 0 ? 'flex' : 'none';

    if (el('subtotal_display'))     el('subtotal_display').textContent     = fmt(subtotal);
    if (el('tax_display'))          el('tax_display').textContent          = fmt(taxAmt);
    if (el('discount_display'))     el('discount_display').textContent     = '-' + fmt(discountAmt);
    if (el('total_display'))        el('total_display').textContent        = fmt(total);
    if (el('mobile_total_display')) el('mobile_total_display').textContent = fmt(total);
}

// ── Enter key on last line item adds a new row ────────────────────
function handleDescEnter(e) {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    const tbody = document.getElementById('items_body_desktop');
    if (!tbody) return;
    const rows = tbody.querySelectorAll('tr');
    const lastDesc = rows[rows.length - 1].querySelector('.item-desc');
    if (e.target === lastDesc) addRow();
}

// ── Attach recalculation listeners ────────────────────────────────
function attachListeners() {
    document.querySelectorAll('.qty, .price, #tax_rate, #currency, #discount_amount').forEach(el => {
        el.removeEventListener('input',  calculateLiveTotals);
        el.removeEventListener('change', calculateLiveTotals);
        el.addEventListener('input',  calculateLiveTotals);
        el.addEventListener('change', calculateLiveTotals);
    });

    // Enter on last description field adds a new row
    document.querySelectorAll('#items_body_desktop .item-desc').forEach(el => {
        el.removeEventListener('keydown', handleDescEnter);
        el.addEventListener('keydown', handleDescEnter);
    });
}
// ── Add row ───────────────────────────────────────────────────────
function addRow() {
    const sym = getCurrencySymbol();

    // Desktop row
    const desktopBody = document.getElementById('items_body_desktop');
    if (desktopBody) {
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td><input type="text" name="description[]" class="item-desc" placeholder="Web Development"></td>
            <td><input type="text" name="details[]" class="item-details" placeholder="40 hours @ $X"></td>
            <td><input type="number" step="0.01" min="0.01" name="quantity[]" class="qty" value="1" inputmode="decimal"></td>
            <td><input type="number" step="0.01" min="0" name="unit_price[]" class="price" value="0.00" inputmode="decimal"></td>
            <td class="right row-amount">${sym}0.00</td>
            <td style="text-align:center;"><button type="button" class="remove-btn" onclick="removeRow(this,'desktop')">×</button></td>
        `;
        desktopBody.appendChild(tr);
	desktopBody.querySelector('tr:last-child .item-desc')?.focus();
    }

    // Mobile card
    const mobileBody = document.getElementById('items_body_mobile');
    if (mobileBody) {
        const card = document.createElement('div');
        card.className = 'item-card-mobile';
        card.innerHTML = `
            <label style="font-size:11px;">Description</label>
            <input type="text" name="description[]" class="item-desc" placeholder="Web Development">
            <label style="font-size:11px;">Details (optional)</label>
            <input type="text" name="details[]" class="item-details" placeholder="40 hours @ $X">
            <div class="item-row-meta">
                <div>
                    <label>Qty</label>
                    <input type="number" step="0.01" min="0.01" name="quantity[]" class="qty" value="1" inputmode="decimal">
                </div>
                <div>
                    <label>Price</label>
                    <input type="number" step="0.01" min="0" name="unit_price[]" class="price" value="0.00" inputmode="decimal">
                </div>
                <div class="row-total">${sym}0.00</div>
                <button type="button" class="remove-btn" onclick="removeRow(this,'mobile')">×</button>
            </div>
        `;
        mobileBody.appendChild(card);
    }

    attachListeners();
    calculateLiveTotals();
    syncItemInputs();
}

// ── Remove row ────────────────────────────────────────────────────
function removeRow(btn, layout) {
    if (layout === 'mobile') {
        const cards = document.querySelectorAll('#items_body_mobile .item-card-mobile');
        if (cards.length <= 1) { alert('An invoice needs at least one item.'); return; }
        btn.closest('.item-card-mobile').remove();
    } else {
        const rows = document.querySelectorAll('#items_body_desktop tr');
        if (rows.length <= 1) { alert('An invoice needs at least one item.'); return; }
        btn.closest('tr').remove();
    }
    calculateLiveTotals();
    syncItemInputs();
}

// ── Client auto-fill ──────────────────────────────────────────────
function fillClient(select) {
    const opt = select.options[select.selectedIndex];
    const clientID = document.getElementById('client_id');
    if (!opt || !opt.value) {
        if (clientID) clientID.value = '';
        return;
    }
    if (clientID) clientID.value = opt.value;

    const map = {
        client_name:    opt.dataset.name,
        client_email:   opt.dataset.email,
        client_address: opt.dataset.address,
        client_city:    opt.dataset.city,
        client_state:   opt.dataset.state,
        client_zip:     opt.dataset.zip,
        client_country: opt.dataset.country,
    };
    for (const [id, val] of Object.entries(map)) {
        const el = document.getElementById(id);
        if (el) el.value = val || '';
    }
}

// ── Auto-Save Draft to localStorage ──────────────────────────────
const DRAFT_KEY = 'pscv_invoice_draft';
const DRAFT_MODE = window.INVOICE_MODE || 'create';

function getFormData() {
    const form = document.getElementById('invoice-form');
    if (!form) return null;
    const data = {};
    form.querySelectorAll('input, textarea, select').forEach(el => {
        if (el.name && el.type !== 'hidden' && el.name !== 'gorilla.csrf.Token') {
            data[el.name] = el.value;
        }
    });
    data._lineItems = [];
    document.querySelectorAll('#items_body_desktop tr').forEach(row => {
        data._lineItems.push({
            description: row.querySelector('.item-desc')?.value || '',
            details:     row.querySelector('.item-details')?.value || '',
            qty:         row.querySelector('.qty')?.value || '1',
            price:       row.querySelector('.price')?.value || '0.00'
        });
    });
    return data;
}

function saveDraft() {
    if (DRAFT_MODE !== 'create') return;
    const data = getFormData();
    if (!data) return;
    data._savedAt = Date.now();
    try {
        localStorage.setItem(DRAFT_KEY, JSON.stringify(data));
        showDraftIndicator();
    } catch (e) {}
}

function restoreDraft() {
    if (DRAFT_MODE !== 'create') return;
    try {
        const raw = localStorage.getItem(DRAFT_KEY);
        if (!raw) return;
        const data = JSON.parse(raw);
        if (data._savedAt && Date.now() - data._savedAt > 86400000) {
            localStorage.removeItem(DRAFT_KEY);
            return;
        }
        const form = document.getElementById('invoice-form');
        if (!form) return;
        form.querySelectorAll('input, textarea, select').forEach(el => {
            if (el.name && el.type !== 'hidden' && el.name !== 'gorilla.csrf.Token') {
                if (data[el.name] !== undefined) el.value = data[el.name];
            }
        });
        if (data._lineItems && data._lineItems.length > 0) {
            const tbody = document.getElementById('items_body_desktop');
            const mobileBody = document.getElementById('items_body_mobile');
            if (!tbody) return;
            while (tbody.rows.length > 1) tbody.deleteRow(tbody.rows.length - 1);
            while (mobileBody && mobileBody.children.length > 1) mobileBody.removeChild(mobileBody.lastChild);
            data._lineItems.forEach((item, i) => {
                if (i > 0) addRow();
                const row = tbody.rows[i];
                if (row) {
                    const d = row.querySelector('.item-desc');
                    const t = row.querySelector('.item-details');
                    const q = row.querySelector('.qty');
                    const p = row.querySelector('.price');
                    if (d) d.value = item.description;
                    if (t) t.value = item.details;
                    if (q) q.value = item.qty;
                    if (p) p.value = item.price;
                }
                if (mobileBody && mobileBody.children[i]) {
                    const card = mobileBody.children[i];
                    const d = card.querySelector('.item-desc');
                    const t = card.querySelector('.item-details');
                    const q = card.querySelector('.qty');
                    const p = card.querySelector('.price');
                    if (d) d.value = item.description;
                    if (t) t.value = item.details;
                    if (q) q.value = item.qty;
                    if (p) p.value = item.price;
                }
            });
        }
        calculateLiveTotals();
        showDraftIndicator('Restored');
    } catch (e) {
        localStorage.removeItem(DRAFT_KEY);
    }
}

function clearDraft() {
    try { localStorage.removeItem(DRAFT_KEY); } catch (e) {}
}

function showDraftIndicator(label) {
    const el = document.getElementById('draft-indicator');
    if (!el) return;
    el.textContent = label ? '📋 ' + label : '💾 Draft saved';
    el.style.opacity = '1';
    setTimeout(() => { el.style.opacity = '0'; }, 2000);
}
// ── Live Validation ──────────────────────────────────────────────
function initValidation() {
    document.querySelectorAll('input[required]').forEach(el => {
        el.addEventListener('blur', () => validateField(el));
        el.addEventListener('input', () => validateField(el));
    });
}

function validateField(el) {
    const val = el.value.trim();
    if (val.length === 0) {
        el.classList.remove('field-valid');
        el.classList.add('field-invalid');
    } else if (val.length >= 2) {
        el.classList.remove('field-invalid');
        el.classList.add('field-valid');
    } else {
        el.classList.remove('field-valid', 'field-invalid');
    }
}

// ── Boot ──────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
    attachListeners();
    restoreDraft();
    calculateLiveTotals();
    initValidation();
    if (DRAFT_MODE === 'create') {
        setInterval(saveDraft, 5000);
    }
    const form = document.getElementById('invoice-form');
    if (form) form.addEventListener('submit', clearDraft);

        // ── Live toggle preview ───────────────────────────────────────
    const toggleLogo       = document.getElementById('toggle_logo');
    const toggleTitle      = document.getElementById('toggle_title');
    const toggleLogoMob    = document.getElementById('toggle_logo_mobile');
    const toggleTitleMob   = document.getElementById('toggle_title_mobile');
    const previewLogo      = document.getElementById('preview_logo');
    const previewTitle     = document.getElementById('preview_title');

    function syncLogo(checked) {
        if (previewLogo)   previewLogo.style.display  = checked ? 'block'  : 'none';
        if (toggleLogo)    toggleLogo.checked    = checked;
        if (toggleLogoMob) toggleLogoMob.checked = checked;
    }

    function syncTitle(checked) {
        if (previewTitle)   previewTitle.style.display = checked ? 'inline' : 'none';
        if (toggleTitle)    toggleTitle.checked    = checked;
        if (toggleTitleMob) toggleTitleMob.checked = checked;
    }

    // Set initial visibility
    syncLogo(toggleLogo?.checked || toggleLogoMob?.checked || false);
    syncTitle(toggleTitle?.checked || toggleTitleMob?.checked || false);

    // Wire both desktop and mobile toggles
    if (toggleLogo)    toggleLogo.addEventListener('change',    function () { syncLogo(this.checked); });
    if (toggleLogoMob) toggleLogoMob.addEventListener('change', function () { syncLogo(this.checked); });
    if (toggleTitle)    toggleTitle.addEventListener('change',    function () { syncTitle(this.checked); });
       if (toggleTitleMob) toggleTitleMob.addEventListener('change', function () { syncTitle(this.checked); });

    // ── Recurring toggle show/hide ───────────────────────────────────
    const toggleRecurring = document.getElementById('toggle_recurring');
    const recurringOptions = document.getElementById('recurring_options');

    function syncRecurring(checked) {
        if (recurringOptions) {
            recurringOptions.style.display = checked ? 'block' : 'none';
        }
    }

    // Set initial state
    if (toggleRecurring) {
        syncRecurring(toggleRecurring.checked);
               toggleRecurring.addEventListener('change', function () {
            syncRecurring(this.checked);
        });
    }
});

// ── Sync responsive item inputs ───────────────────────────────────
// CSS hides desktop table at <=639px and shows mobile cards.
// But CSS display:none does NOT stop inputs from submitting.
// We disable inputs in the hidden layout so only the visible
// layout's values are posted to the server.
const MOBILE_BREAKPOINT = 639;

function syncItemInputs() {
    const isMobile = window.innerWidth <= MOBILE_BREAKPOINT;

    // Disable the hidden layout's inputs, enable the visible one
    document.querySelectorAll('#items_body_desktop input').forEach(el => {
        el.disabled = isMobile;
    });
    document.querySelectorAll('#items_body_mobile input').forEach(el => {
        el.disabled = !isMobile;
    });
}

// Run on load and on every resize
syncItemInputs();
window.addEventListener('resize', syncItemInputs);

// Re-run after form submit fires (safety net)
const _invoiceForm = document.getElementById('invoice-form');
if (_invoiceForm) {
    _invoiceForm.addEventListener('submit', () => {
        syncItemInputs();
    });
}

// ── Logo Position Selector ────────────────────────────────────────
function setLogoPos(pos) {
    // Update hidden input
    const input = document.getElementById('logo_position_input');
    if (input) input.value = pos;

    // Update all button states (desktop + mobile)
    document.querySelectorAll('.logo-pos-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.pos === pos);
    });

    // Update live preview
   const previewLogo = document.getElementById('preview_logo');
   if (previewLogo) {
        previewLogo.style.textAlign = pos;
    }
    }

// Set initial state on load
document.addEventListener('DOMContentLoaded', () => {
    const input = document.getElementById('logo_position_input');
    if (input) setLogoPos(input.value || 'left');
});
