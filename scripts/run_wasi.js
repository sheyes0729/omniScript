const fs = require('fs');
const path = require('path');
const WabtModule = require('wabt');
const { WASI } = require('wasi');
const { Worker, isMainThread, parentPort, workerData } = require('worker_threads');

if (isMainThread) {
    async function run() {
        if (process.argv.length < 3) {
            console.error("Usage: node run_wasi.js <file.wat> [args...]");
            process.exit(1);
        }

        const watPath = process.argv[2];
        const watContent = fs.readFileSync(watPath, 'utf8');

        const wabt = await WabtModule();
        const module = wabt.parseWat(path.basename(watPath), watContent, { threads: true });
        // Enable threads feature
        const { buffer } = module.toBinary({ features: { threads: true } });

        // Create shared memory
        // Initial: 100 pages (6.4MB), Max: 1000 pages (64MB)
        const sharedMemory = new WebAssembly.Memory({ initial: 100, maximum: 1000, shared: true });

        // Initialize Heap Pointer at 1020 to 10240
        const memView = new DataView(sharedMemory.buffer);
        memView.setInt32(1020, 10240, true);

        const wasi = new WASI({
            version: 'preview1',
            args: process.argv,
            env: process.env,
            preopens: {
                '.': '.'
            }
        });

        // Function map to be filled after instantiation
        let instanceExports = null;

        const importObject = {
            wasi_snapshot_preview1: wasi.wasiImport,
            env: {
                memory: sharedMemory,
                thread_spawn: (funcNamePtr, argsArrPtr) => {
                    // Read function name from shared memory
                    const memBuffer = sharedMemory.buffer;
                    const memView = new Uint8Array(memBuffer);
                    
                    let name = "";
                    let ptr = funcNamePtr;
                    while (memView[ptr] !== 0) {
                        name += String.fromCharCode(memView[ptr]);
                        ptr++;
                    }
                    
                    // Allocate stack for new thread (1MB)
                    const int32View = new Int32Array(sharedMemory.buffer);
                    const heapPtrIndex = 255; // 1020 / 4
                    const stackSize = 1024 * 1024;
                    const stackBase = Atomics.add(int32View, heapPtrIndex, stackSize);
                    
                    // Create Worker
                    const worker = new Worker(__filename, {
                        workerData: {
                            bytecode: buffer,
                            memory: sharedMemory,
                            funcName: name,
                            argsPtr: argsArrPtr,
                            stackBase: stackBase
                        }
                    });
                    
                    worker.on('error', (err) => console.error(`[Worker Error]`, err));
                    // worker.on('exit', (code) => console.log(`[Worker Exit] code ${code}`));
                    
                    return 1; // Thread ID (dummy)
                },
                host_to_int: (val) => val,
                // Add other required imports if missing from compilation
                print: (ptr) => {
                    const memBuffer = sharedMemory.buffer;
                    const memView = new Uint8Array(memBuffer);
                    let str = "";
                    let i = ptr;
                    while (memView[i] !== 0) {
                        str += String.fromCharCode(memView[i]);
                        i++;
                    }
                    console.log(str);
                },
                print_int: (val) => {
                    console.log(val);
                },
                host_get_global: () => 0,
                host_get: () => 0,
                host_set: () => 0,
                host_call: () => 0,
                host_from_int: () => 0,
                host_from_string: () => 0,
            }
        };

        const { instance } = await WebAssembly.instantiate(buffer, importObject);
        instanceExports = instance.exports;
        
        // wasi.start(instance);
        // Use initialize for Reactor model (since we export _initialize)
        wasi.initialize(instance);
         // Call main for reactor model
         if (instance.exports.main) {
             instance.exports.main();
         } else if (instance.exports._start) {
             instance.exports._start();
         }
    }

    run().catch(err => {
        // Check for WASI exit (it throws an error to exit)
        // The error object might be internal, check toString() or similar
        if (err.toString().includes("ExitStatus") || err.toString().includes("kExitCode")) {
             // Normal exit
             return;
        }
        if (typeof err === 'object' && err !== null && 'code' in err && typeof err.code === 'number') {
             process.exit(err.code);
        }
        
        console.error("Runtime Error:", err);
        process.exit(1);
    });

} else {
    // Worker Thread
    async function workerRun() {
        const { bytecode, memory, funcName, argsPtr, stackBase } = workerData;
        
        const wasi = new WASI({
            version: 'preview1',
            args: [], // Worker has no args
            env: process.env,
            preopens: { '.': '.' }
        });
        
        const importObject = {
            wasi_snapshot_preview1: wasi.wasiImport,
            env: {
                memory: memory,
                thread_spawn: () => 0, // Workers can't spawn (for now)
                print: (ptr) => {
                    const memBuffer = memory.buffer;
                    const memView = new Uint8Array(memBuffer);
                    let str = "";
                    let i = ptr;
                    while (memView[i] !== 0) {
                        str += String.fromCharCode(memView[i]);
                        i++;
                    }
                    console.log(str);
                },
                host_to_int: (val) => val,
                host_get_global: () => 0,
                host_get: () => 0,
                host_set: () => 0,
                host_call: () => 0,
                host_from_int: () => 0,
                host_from_string: () => 0,
            }
        };
        
        const { instance } = await WebAssembly.instantiate(bytecode, importObject);
        
        // Set stack pointer for this thread
        if (instance.exports._set_stack_pointer) {
            instance.exports._set_stack_pointer(stackBase);
        } else {
            console.error("[Worker] _set_stack_pointer export missing!");
        }

        // Initialize WASI (Reactor model)
        wasi.initialize(instance);
        
        // Find function export
        const func = instance.exports[funcName];
        if (!func) {
            console.error(`[Worker] Function ${funcName} not found in exports`);
            return;
        }
        
        // We need to unpack arguments from argsPtr (Array)
        // Array layout: [len, cap, data_ptr]
        // data[i] = value
        
        const memView = new DataView(memory.buffer);
        const len = memView.getInt32(argsPtr, true); // Little endian
        const dataPtr = memView.getInt32(argsPtr + 8, true);
        
        const args = [];
        for (let i = 0; i < len; i++) {
            const val = memView.getInt32(dataPtr + i * 4, true);
            args.push(val);
        }
        
        // console.log(`[Worker] Running ${funcName} with args:`, args);
        func(...args);
    }
    
    workerRun().catch(err => console.error(err));
}
