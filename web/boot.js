// The engine and matching Go runtime are embedded; evaluation makes no requests.
(async () => {
  const fail = (error) => {
    window.storepathEngineError = error instanceof Error ? error.message : String(error);
    window.dispatchEvent(new CustomEvent("storepath-error", {
      detail: { message: window.storepathEngineError },
    }));
  };
  try {
    if (typeof WebAssembly !== "object" || typeof Go !== "function") {
      throw new Error("This demo needs a modern browser with WebAssembly enabled.");
    }
    const encoded = document.getElementById("storepath-wasm").textContent.trim();
    const bytes = Uint8Array.from(atob(encoded), (character) => character.charCodeAt(0));
    const go = new Go();
    const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(instance).catch(fail);
    if (typeof window.storepathEvaluate !== "function") {
      throw new Error("The storage engine did not start. Reload the demo to try again.");
    }
    window.dispatchEvent(new CustomEvent("storepath-ready"));
  } catch (error) {
    fail(error);
  }
})();
