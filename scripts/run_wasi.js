const fs = require('fs');
const path = require('path');
const WabtModule = require('wabt');
const { WASI } = require('wasi');

async function run() {
    if (process.argv.length < 3) {
        console.error("Usage: node run_wasi.js <file.wat>");
        process.exit(1);
    }

    const watPath = process.argv[2];
    const watContent = fs.readFileSync(watPath, 'utf8');

    const wabt = await WabtModule();
    const module = wabt.parseWat(path.basename(watPath), watContent);
    const { buffer } = module.toBinary({});

    const wasi = new WASI({
        version: 'preview1',
        args: process.argv,
        env: process.env,
        preopens: {
            '.': '.' // Map current directory to current directory in WASI
        }
    });

    const importObject = {
        wasi_snapshot_preview1: wasi.wasiImport
    };

    const { instance } = await WebAssembly.instantiate(buffer, importObject);
    
    // WASI start handles _start execution and setup
    wasi.start(instance);
}

run().catch(err => {
    console.error("Runtime Error:", err);
    process.exit(1);
});
