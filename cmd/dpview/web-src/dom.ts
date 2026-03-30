/** Throws when a required element id is missing from the page shell. */
export function requiredElement<T extends HTMLElement>(id: string): T {
    const element = document.getElementById(id);
    if (!element) {
        throw new Error(`Missing required element #${id}`);
    }
    return element as T;
}

/** Throws when a required selector is missing from the page shell. */
export function requiredSelector<T extends Element>(selector: string): T {
    const element = document.querySelector(selector);
    if (!element) {
        throw new Error(`Missing required element ${selector}`);
    }
    return element as T;
}
