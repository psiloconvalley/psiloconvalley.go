(() => {
    "use strict";

    const COUNTRY_DATA_URL = "/static/data/country-region-data.json";
    let countryRegionData = [];

    const fallbackCountryRegionData = [
        {
            countryName: "United States",
            countryShortCode: "US",
            regions: [
                "Alabama", "Alaska", "Arizona", "Arkansas", "California",
                "Colorado", "Connecticut", "Delaware", "Florida", "Georgia",
                "Hawaii", "Idaho", "Illinois", "Indiana", "Iowa",
                "Kansas", "Kentucky", "Louisiana", "Maine", "Maryland",
                "Massachusetts", "Michigan", "Minnesota", "Mississippi", "Missouri",
                "Montana", "Nebraska", "Nevada", "New Hampshire", "New Jersey",
                "New Mexico", "New York", "North Carolina", "North Dakota", "Ohio",
                "Oklahoma", "Oregon", "Pennsylvania", "Rhode Island", "South Carolina",
                "South Dakota", "Tennessee", "Texas", "Utah", "Vermont",
                "Virginia", "Washington", "West Virginia", "Wisconsin", "Wyoming"
            ].map(name => ({ name }))
        },
        {
            countryName: "Canada",
            countryShortCode: "CA",
            regions: [
                "Alberta", "British Columbia", "Manitoba", "New Brunswick",
                "Newfoundland and Labrador", "Nova Scotia", "Ontario",
                "Prince Edward Island", "Quebec", "Saskatchewan",
                "Northwest Territories", "Nunavut", "Yukon"
            ].map(name => ({ name }))
        },
        {
            countryName: "Mexico",
            countryShortCode: "MX",
            regions: [
                "Jalisco", "Nuevo León", "CDMX", "Yucatán", "Baja California",
                "Chihuahua", "Guanajuato", "Puebla", "Veracruz", "Oaxaca"
            ].map(name => ({ name }))
        },
        {
            countryName: "United Kingdom",
            countryShortCode: "GB",
            regions: [
                "England", "Scotland", "Wales", "Northern Ireland"
            ].map(name => ({ name }))
        },
        {
            countryName: "Other",
            countryShortCode: "OTHER",
            regions: [
                { name: "Other / Not listed" }
            ]
        }
    ];

    function qs(selector, root = document) {
        return root.querySelector(selector);
    }

    function qsa(selector, root = document) {
        return Array.from(root.querySelectorAll(selector));
    }

    function parseNumber(value, fallback = 0) {
        const cleaned = String(value ?? "").replace(/,/g, "");
        const number = Number.parseFloat(cleaned);
        if (!Number.isFinite(number)) return fallback;
        return number;
    }

    function normalizeCountryRegionData(data) {
        if (!Array.isArray(data)) return fallbackCountryRegionData;

        return data
            .filter(country => country.countryName && country.countryShortCode)
            .map(country => ({
                countryName: country.countryName,
                countryShortCode: country.countryShortCode,
                regions: Array.isArray(country.regions)
                    ? country.regions.map(region => ({
                        name: region.name,
                        shortCode: region.shortCode || ""
                    }))
                    : []
            }))
            .sort((a, b) => a.countryName.localeCompare(b.countryName));
    }

    async function loadCountryRegionData() {
        try {
            const response = await fetch(COUNTRY_DATA_URL, { cache: "force-cache" });
            if (!response.ok) throw new Error("Could not load country/region JSON");
            const data = await response.json();
            countryRegionData = normalizeCountryRegionData(data);
        } catch (error) {
            console.warn("Using fallback country data:", error);
            countryRegionData = fallbackCountryRegionData;
        }
    }

    // =========================================================
    // SERVER-DRIVEN UI: COUNTRY & REGION HYDRATION
    // The Backend is the Single Source of Truth.
    // We read the `data-selected` attributes injected by Go.
    // =========================================================

    function populateCountrySelect(selectID) {
        const select = document.getElementById(selectID);
        if (!select) return;

        // 1. Read the server's truth. Fallback to "US" only if server sends nothing.
        const serverSelected = select.dataset.selected || "US"; 
        
        select.innerHTML = "";

        countryRegionData.forEach(country => {
            const option = document.createElement("option");
            
            // 2. CRITICAL FIX: Use ISO Short Code as the value to match Go backend
            option.value = country.countryShortCode; 
            option.textContent = country.countryName;
            option.dataset.code = country.countryShortCode;

            // 3. Hydrate from server state
            if (country.countryShortCode === serverSelected) {
                option.selected = true;
            }

            select.appendChild(option);
        });
    }

    function populateRegionSelect(countrySelectID, regionSelectID) {
        const countrySelect = document.getElementById(countrySelectID);
        const regionSelect = document.getElementById(regionSelectID);

        if (!countrySelect || !regionSelect) return;

        // 1. Read the server's truth for the region
        const serverSelectedRegion = regionSelect.dataset.selected || "";
        const selectedCountryCode = countrySelect.value;

        const country = countryRegionData.find(item => item.countryShortCode === selectedCountryCode);
        regionSelect.innerHTML = "";

        if (!country || !country.regions || country.regions.length === 0) {
            const option = document.createElement("option");
            option.value = "";
            option.textContent = "Not applicable";
            regionSelect.appendChild(option);
            return;
        }

        let selectedWasSet = false;

        country.regions.forEach(region => {
            const option = document.createElement("option");
            option.value = region.name;
            option.textContent = region.name;

            // 2. Hydrate from server state (case-insensitive match)
            if (serverSelectedRegion && region.name.toLowerCase() === serverSelectedRegion.toLowerCase()) {
                option.selected = true;
                selectedWasSet = true;
            }

            regionSelect.appendChild(option);
        });

        if (!selectedWasSet && regionSelect.options.length > 0) {
            regionSelect.selectedIndex = 0;
        }
    }

    function setupCountryRegionPair(countrySelectID, regionSelectID) {
        const countrySelect = document.getElementById(countrySelectID);
        const regionSelect = document.getElementById(regionSelectID);

        if (!countrySelect || !regionSelect) return;

        // Initial hydration based on server-rendered DOM
        populateRegionSelect(countrySelectID, regionSelectID);

        // User interaction: When country changes, clear the server-selected 
        // region so it doesn't force "California" when switching to "Canada"
        countrySelect.addEventListener("change", () => {
            regionSelect.dataset.selected = ""; 
            populateRegionSelect(countrySelectID, regionSelectID);
        });
    }

    function setupCountryDropdowns() {
        populateCountrySelect("company_country");
        populateCountrySelect("client_country");

        setupCountryRegionPair("company_country", "company_state");
        setupCountryRegionPair("client_country", "client_state");
    }

    // =========================================================
    // CURRENCY & MATH UI
    // =========================================================

    function currentSymbol() {
        const select = document.getElementById("currency");
        if (!select) return "$";
        const selected = select.options[select.selectedIndex];
        if (!selected) return "$";
        return selected.dataset.symbol || "$";
    }

    function money(value) {
        return currentSymbol() + value.toFixed(2);
    }

    function updateHiddenDescriptions() {
        const rows = qsa("#items_body tr");
        rows.forEach(row => {
            const itemNameInput = qs(".item-name", row);
            const itemDetailInput = qs(".item-detail", row);
            const hiddenDescriptionInput = qs(".desc-hidden", row);

            if (!itemNameInput || !itemDetailInput || !hiddenDescriptionInput) return;

            const itemName = itemNameInput.value.trim();
            const itemDetail = itemDetailInput.value.trim();

            if (itemName && itemDetail) {
                hiddenDescriptionInput.value = itemName + " — " + itemDetail;
            } else if (itemName) {
                hiddenDescriptionInput.value = itemName;
            } else {
                hiddenDescriptionInput.value = itemDetail;
            }
        });
    }

    function calculateTotals() {
        let subtotal = 0;
        const rows = qsa("#items_body tr");

        rows.forEach(row => {
            const qtyInput = qs(".qty", row);
            const priceInput = qs(".price", row);
            const amountCell = qs(".row-amount", row);

            if (!qtyInput || !priceInput || !amountCell) return;

            const qty = parseNumber(qtyInput.value, 0);
            const price = parseNumber(priceInput.value, 0);
            const amount = qty * price;

            amountCell.innerText = money(amount);
            subtotal += amount;
        });

        const taxRateInput = document.getElementById("tax_rate");
        const taxRate = taxRateInput ? parseNumber(taxRateInput.value, 0) : 0;

        const taxAmount = subtotal * (taxRate / 100);
        const total = subtotal + taxAmount;

        const subtotalEl = document.getElementById("subtotal");
        const taxAmountEl = document.getElementById("tax_amount");
        const totalEl = document.getElementById("total");

        if (subtotalEl) subtotalEl.innerText = money(subtotal);
        if (taxAmountEl) taxAmountEl.innerText = money(taxAmount);
        if (totalEl) totalEl.innerText = money(total);

        updateHiddenDescriptions();
    }

    function addRow() {
        const tbody = document.getElementById("items_body");
        if (!tbody) return;

        const row = document.createElement("tr");
        row.innerHTML = `
            <td><input type="text" class="item-name" placeholder="Service or product" required></td>
            <td>
                <input type="text" class="item-detail" placeholder="Short description">
                <input type="hidden" name="description" class="desc-hidden">
            </td>
            <td><input type="number" step="0.01" name="quantity" value="1" class="qty"></td>
            <td><input type="number" step="0.01" name="unit_price" value="0" class="price"></td>
            <td class="right amount row-amount">${money(0)}</td>
            <td><button type="button" class="remove-btn" onclick="removeRow(this)">×</button></td>
        `;
        tbody.appendChild(row);
        calculateTotals();
    }

    function removeRow(button) {
        const rows = qsa("#items_body tr");
        if (rows.length <= 1) return;
        const row = button.closest("tr");
        if (row) row.remove();
        calculateTotals();
    }

    function bindFormEvents() {
        const form = document.getElementById("invoice-form");
        if (!form) return;

        form.addEventListener("input", event => {
            if (
                event.target.matches(".qty") ||
                event.target.matches(".price") ||
                event.target.matches(".item-name") ||
                event.target.matches(".item-detail") ||
                event.target.matches("#tax_rate")
            ) {
                calculateTotals();
            }
        });

        form.addEventListener("change", event => {
            if (event.target.matches("#currency")) {
                calculateTotals();
            }
        });

        form.addEventListener("submit", () => {
            updateHiddenDescriptions();
        });
    }

    async function initInvoiceForm() {
        await loadCountryRegionData();
        setupCountryDropdowns();
        bindFormEvents();
        calculateTotals();
    }

    window.addRow = addRow;
    window.removeRow = removeRow;

    window.addEventListener("DOMContentLoaded", initInvoiceForm);
})();
