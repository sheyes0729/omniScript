try {
    const { WASI } = require('wasi');
    if (WASI) {
        console.log("WASI is available");
    } else {
        console.log("WASI is NOT available");
    }
} catch (e) {
    console.log("WASI require failed:", e.message);
}
