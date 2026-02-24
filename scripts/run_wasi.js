const fs = require('fs');
const path = require('path');
const http = require('http');
const net = require('net');
const dgram = require('dgram');
const WabtModule = require('wabt');
const { WASI } = require('wasi');
const { Worker, isMainThread, parentPort, workerData } = require('worker_threads');

const activeWorkers = [];

// Handle Manager for Host Objects
class HandleManager {
    constructor() {
        this.handles = new Map();
        this.nextId = 1;
        // Pre-register global modules
        this.register(http, "http");
        this.register(net, "net");
        this.register(dgram, "dgram");
    }

    register(obj, id = null) {
        if (id) {
            // Check if fixed ID is taken (simple override for now)
            this.handles.set(id, obj);
            return id; // Return as string? Wait, WASM handles are i32.
            // We need string-to-id mapping for globals, but handle is i32.
            // Let's use negative IDs for globals? Or just keep a separate map.
        }
        const handle = this.nextId++;
        this.handles.set(handle, obj);
        return handle;
    }

    get(handle) {
        return this.handles.get(handle);
    }

    remove(handle) {
        this.handles.delete(handle);
    }
}

const handleMgr = new HandleManager();
const globalModules = {
    "http": http,
    "net": net,
    "dgram": dgram
};

function readString(memory, ptr) {
    const memView = new Uint8Array(memory.buffer);
    let str = "";
    let i = ptr;
    while (memView[i] !== 0) {
        str += String.fromCharCode(memView[i]);
        i++;
    }
    return str;
}

function writeString(memory, ptr, str) {
    // Basic implementation: assumes buffer is large enough or allocated
    // Ideally we should allocate new string in WASM, but here we just need to return primitive values?
    // For now, let's just support returning integer handles or primitives.
    // Returning strings from host to WASM requires `malloc` export from WASM.
}

if (isMainThread) {
    async function run() {
        if (process.argv.length < 3) {
            console.error("Usage: node run_wasi.js <file.wat> [args...]");
            process.exit(1);
        }

        const watPath = process.argv[2];
        const watContent = fs.readFileSync(watPath, 'utf8');

        const wabt = await WabtModule();
        const module = wabt.parseWat(path.basename(watPath), watContent, { threads: true, exceptions: true });
        // Enable threads feature
        const { buffer } = module.toBinary({ features: { threads: true, exceptions: true } });

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
                    
                    // console.log(`[Host] Spawning thread for function: ${name}`);

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
                    
                    activeWorkers.push(worker);
                    // console.log("Spawned worker", worker.threadId);
                    return 1;
                },
                host_to_int: (val) => val,
                // Add other required imports if missing from compilation
                print: (ptr) => {
                    console.log(readString(sharedMemory, ptr));
                },
                print_int: (val) => {
                    console.log(val);
                },
                console_log_int: (val) => {
                    process.stdout.write(val.toString());
                },
                console_log_char: (val) => {
                    process.stdout.write(String.fromCharCode(val));
                },
                console_log_str: (ptr) => {
                    const str = readString(sharedMemory, ptr);
                    process.stdout.write(str);
                },
                host_get_global: (namePtr) => {
                    const name = readString(sharedMemory, namePtr);
                    if (globalModules[name]) {
                        return handleMgr.register(globalModules[name]);
                    }
                    return 0;
                },
                host_get: (handle, propPtr) => {
                    const obj = handleMgr.get(handle);
                    if (!obj) return 0;
                    const prop = readString(sharedMemory, propPtr);
                    // console.log("host_get:", handle, prop);
                    const val = obj[prop];
                    if (typeof val === 'function') {
                        // Bind function to object
                        return handleMgr.register(val.bind(obj));
                    }
                    if (typeof val === 'object' && val !== null) {
                        return handleMgr.register(val);
                    }
                    return val; // Return primitive? If string, need host_from_string
                },
                host_set: (handle, propPtr, valHandle) => {
                    const obj = handleMgr.get(handle);
                    if (!obj) return;
                    const prop = readString(sharedMemory, propPtr);
                    // Value might be handle or primitive.
                    // For MVP, assume valHandle is a handle if we have a way to know?
                    // Actually, host_set takes i32. If it's a handle, we get object.
                    // If it's primitive int, we get int.
                    // But we don't know type here.
                    // Let's assume valHandle is just the value for int/bool.
                    // For string, we used host_from_string which likely returns a handle to a wrapper?
                    // Or we just passed pointer?
                    // In compiler.go: host_from_string takes i32 (ptr) -> result i32 (handle).
                    
                    const val = handleMgr.get(valHandle);
                    obj[prop] = val !== undefined ? val : valHandle;
                },
                host_call: (handle, methodPtr, argsPtr, argsCount) => {
                    let func;
                    let thisArg;

                    // If methodPtr is provided, look up method on object
                    if (methodPtr !== 0) {
                        const obj = handleMgr.get(handle);
                        if (!obj) return 0;
                        const methodName = readString(sharedMemory, methodPtr);
                        func = obj[methodName];
                        thisArg = obj;
                        // console.log(`Calling method ${methodName} on object`, obj);
                    } else {
                        // Direct function call (not supported by compiler yet for TypeHost variables, but good to have)
                        func = handleMgr.get(handle);
                        thisArg = null; // Or global?
                    }
                    
                    if (typeof func !== 'function') return 0;
                    
                    const memView = new DataView(sharedMemory.buffer);
                    const args = [];
                    for (let i = 0; i < argsCount; i++) {
                        const val = memView.getInt32(argsPtr + i * 4, true);
                        // Resolve handles if possible
                        let obj = handleMgr.get(val);
                        if (obj !== undefined) {
                            // Unwrap primitive wrappers
                            if (obj instanceof String) obj = obj.toString();
                            else if (obj instanceof Number) obj = obj.valueOf();
                            else if (obj instanceof Boolean) obj = obj.valueOf();
                            args.push(obj);
                        } else {
                            args.push(val);
                        }
                    }
                    
                    try {
                        // console.log("Calling host function:", func.name || "anonymous", "Args:", args);
                        const result = func.apply(thisArg, args);
                        // console.log("Result:", result);
                        if (typeof result === 'object' && result !== null) {
                            return handleMgr.register(result);
                        }
                        if (typeof result === 'string') {
                             // String return needs to be handled.
                             // For now, return handle to string object?
                             // Or we need host_to_string?
                             // Compiler expects i32.
                             return handleMgr.register(new String(result)); 
                        }
                        return result;
                    } catch (e) {
                        console.error("Host call error:", e);
                        return 0;
                    }
                },
                host_from_int: (val) => {
                    return val; // Pass through int
                },
                host_from_string: (ptr) => {
                    const str = readString(sharedMemory, ptr);
                    return handleMgr.register(new String(str)); // Wrap string as object handle
                },
                host_to_int: (handle) => {
                    const val = handleMgr.get(handle);
                    if (val instanceof String) return parseInt(val.toString());
                    return Number(val);
                },
            }
        };

        const { instance } = await WebAssembly.instantiate(buffer, importObject);
        instanceExports = instance.exports;
        
        // console.log("Instance instantiated. Exports:", Object.keys(instance.exports));

        // Use initialize for Reactor model (since we export _initialize)
        wasi.initialize(instance);
         // Call main for reactor model
         if (instance.exports.main) {
             // console.log("Calling main...");
             instance.exports.main();
             // console.log("main returned.");
         } else if (instance.exports._start) {
             // console.log("Calling _start...");
             instance.exports._start();
         } else {
             // console.log("No entry point found.");
         }
         
         // Wait a bit for workers to finish tasks, then exit
         // In a real app, we might wait for explicit shutdown.
         // For tests, we assume main spawns and we wait a bit.
         setTimeout(() => {
             console.log("Terminating workers... count:", activeWorkers.length);
             for (const w of activeWorkers) {
                 w.terminate();
             }
             process.exit(0);
         }, 2000); // Wait 2 seconds
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
                    // console.log(readString(memory, ptr));
                    fs.writeSync(1, readString(memory, ptr) + "\n");
                },
                print_int: (val) => {
                    // console.log("[Worker PrintInt]", val);
                    // process.stdout.write(`[Worker PrintInt] ${val}\n`);
                    fs.writeSync(1, `${val}\n`);
                },
                console_log_str: (ptr) => {
                    const str = readString(memory, ptr);
                    fs.writeSync(1, str);
                },
                console_log_int: (val) => {
                    fs.writeSync(1, val.toString());
                },
                console_log_char: (val) => {
                    fs.writeSync(1, String.fromCharCode(val));
                },
                host_to_int: (val) => val,
                host_get_global: (namePtr) => {
                    // Worker threads don't share handles yet.
                    // For MVP, workers can't access host objects unless passed explicitly.
                    // Or we need SharedArrayBuffer based handle map?
                    return 0; 
                },
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
        let args = [];
        
        if (argsPtr !== 0) {
            const len = memView.getInt32(argsPtr, true); // Little endian
            const dataPtr = memView.getInt32(argsPtr + 8, true);
            
            for (let i = 0; i < len; i++) {
                const val = memView.getInt32(dataPtr + i * 4, true);
                args.push(val);
            }
        }
        
        // process.stdout.write(`[Worker] Running ${funcName} with args: ${args}\n`);
        // process.stdout.write(`[Worker] Func type: ${typeof func}\n`);
        try {
            func(...args);
        } catch (e) {
            console.error(`[Worker] Error running ${funcName}:`, e);
        }
    }
    
    workerRun().catch(err => console.error(err));
}
