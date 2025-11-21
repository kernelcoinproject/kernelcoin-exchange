/**
 * Tabs Module
 * Handles dynamic loading of tab content from separate HTML files
 */

const tabContainer = document.getElementById('tabContainer');

const tabs = {
    auth: 'auth.html',
    exchange: 'exchange.html',
    trades: 'trades.html',
    balances: 'balances.html',
    wallet: 'wallet.html',
    admin: 'admin.html'
};

let loadedTabs = new Set();

/**
 * Load a tab's HTML content from disk
 */
async function loadTab(tabName) {
    if (loadedTabs.has(tabName) || document.getElementById(tabName)) {
        return; // Already loaded
    }

    const tabFile = tabs[tabName];
    if (!tabFile) {
        console.error(`Tab ${tabName} not found`);
        return;
    }

    try {
        const response = await fetch(tabFile);
        if (!response.ok) {
            throw new Error(`Failed to load ${tabFile}`);
        }
        const html = await response.text();
        tabContainer.innerHTML += html;
        loadedTabs.add(tabName);
    } catch (error) {
        console.error(`Error loading tab ${tabName}:`, error);
    }
}

/**
 * Initialize tabs - conditionally load auth tab on page load
 */
async function initializeTabs() {
    // Don't load auth tab initially - let checkSessionOnLoad decide
}

/**
 * Load a tab on demand when user clicks tab button
 */
async function loadTabOnDemand(tabName) {
    await loadTab(tabName);
}

// Initialize tabs when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initializeTabs);
} else {
    initializeTabs();
}
