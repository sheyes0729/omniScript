class OmniRuntime {
    constructor() {
        this.memory = null;
        this.heap = new Map();
        this.nextHandle = 1; // 0 is null
        this.textDecoder = new TextDecoder('utf-8');
        
        // Define global object (browser: window, node: global)
        this.global = typeof window !== 'undefined' ? window : global;
    }

    setMemory(memory) {
        this.memory = memory;
    }

    readString(ptr) {
        if (!this.memory) return "";
        const buffer = new Uint8Array(this.memory.buffer);
        let end = ptr;
        while (buffer[end] !== 0) end++;
        return this.textDecoder.decode(buffer.subarray(ptr, end));
    }

    storeObject(obj) {
        if (obj === null || obj === undefined) return 0;
        const handle = this.nextHandle++;
        this.heap.set(handle, obj);
        return handle;
    }

    getObject(handle) {
        if (handle === 0) return null;
        return this.heap.get(handle);
    }

    getImports() {
        return {
            env: {
                print: (ptr) => {
                    const str = this.readString(ptr);
                    console.log("[Omni]", str);
                    if (typeof document !== 'undefined') {
                        const output = document.getElementById('output');
                        if (output) {
                            const div = document.createElement('div');
                            div.textContent = str;
                            output.appendChild(div);
                        }
                    }
                },
                
                // Generic Host Interop
                
                // $host_get_global(namePtr) -> handle
                host_get_global: (namePtr) => {
                    const name = this.readString(namePtr);
                    const val = this.global[name];
                    return this.storeObject(val);
                },
                
                // $host_get(handle, propPtr) -> handle
                host_get: (handle, propPtr) => {
                    const obj = this.getObject(handle);
                    const prop = this.readString(propPtr);
                    if (obj === undefined || obj === null) return 0;
                    return this.storeObject(obj[prop]);
                },
                
                // $host_set(handle, propPtr, valueHandle)
                host_set: (handle, propPtr, valueHandle) => {
                    const obj = this.getObject(handle);
                    const prop = this.readString(propPtr);
                    const val = this.getObject(valueHandle);
                    if (obj !== undefined && obj !== null) {
                        obj[prop] = val;
                    }
                },
                
                // $host_call(handle, methodPtr, argsPtr, argsLen) -> handle
                host_call: (handle, methodPtr, argsPtr, argsLen) => {
                    const obj = this.getObject(handle);
                    const args = [];
                    
                    if (argsLen > 0) {
                        const buffer = new Int32Array(this.memory.buffer);
                        // argsPtr is byte offset, need index for Int32Array (ptr / 4)
                        const startIdx = argsPtr / 4;
                        for (let i = 0; i < argsLen; i++) {
                            const argHandle = buffer[startIdx + i];
                            args.push(this.getObject(argHandle));
                        }
                    }
                    
                    let result;
                    if (methodPtr === 0) {
                        // Call obj as function
                        if (typeof obj === 'function') {
                            result = obj.apply(this.global, args);
                        }
                    } else {
                        // Call method on obj
                        const method = this.readString(methodPtr);
                        if (obj && typeof obj[method] === 'function') {
                            result = obj[method].apply(obj, args);
                        }
                    }
                    
                    return this.storeObject(result);
                },
                
                // Converters
                
                // $host_from_int(i32) -> handle
                host_from_int: (val) => {
                    return this.storeObject(val);
                },
                
                // $host_from_string(ptr) -> handle
                host_from_string: (ptr) => {
                    const str = this.readString(ptr);
                    return this.storeObject(str);
                },
                
                // $host_to_int(handle) -> i32
                host_to_int: (handle) => {
                    const val = this.getObject(handle);
                    return typeof val === 'number' ? val : 0;
                }
            }
        };
    }
}
