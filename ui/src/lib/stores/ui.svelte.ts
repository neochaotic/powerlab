import { goto } from '$app/navigation';

let isTerminalOpen = $state(false);
let searchTriggered = $state(0); // Increment to trigger focus/action

export const ui = {
	get isTerminalOpen() { return isTerminalOpen; },
	set isTerminalOpen(v: boolean) { isTerminalOpen = v; },
	
	get searchTriggered() { return searchTriggered; },
	
	openSearch() {
		searchTriggered++;
		if (window.location.pathname !== '/apps') {
			goto('/apps');
		}
	},
	
	openTerminal() {
		isTerminalOpen = true;
	}
};
