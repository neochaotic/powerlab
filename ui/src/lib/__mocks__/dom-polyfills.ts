/**
 * jsdom doesn't implement Web Animations API. Svelte's transition:fade /
 * transition:scale call element.animate(...). Without these stubs every
 * component test that uses Svelte transitions throws
 * `TypeError: element.animate is not a function`.
 *
 * The stubs return a no-op Animation-like object. We don't care about
 * the visual transition in tests — we only assert on rendered DOM
 * after the component settles.
 */
if (typeof Element !== 'undefined' && !Element.prototype.animate) {
	Element.prototype.animate = function () {
		const noopAnimation = {
			cancel: () => {},
			finish: () => {},
			pause: () => {},
			play: () => {},
			reverse: () => {},
			finished: Promise.resolve(),
			onfinish: null,
			oncancel: null,
			playState: 'finished',
			currentTime: 0,
			startTime: 0,
			effect: null,
			id: '',
			pending: false,
			playbackRate: 1,
			ready: Promise.resolve(),
			timeline: null,
			addEventListener: () => {},
			removeEventListener: () => {},
			dispatchEvent: () => true
		};
		return noopAnimation as unknown as Animation;
	};
}
