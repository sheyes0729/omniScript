package compiler

import (
	"bytes"
	"fmt"
	"strings"
	"omniScript/pkg/ast"
)

type DataType string

const (
	TypeInt     DataType = "int"
	TypeString  DataType = "string"
	TypeVoid    DataType = "void"
	TypeBool    DataType = "bool"
	TypeArray   DataType = "array"
	TypeMap     DataType = "map"
	TypeHost    DataType = "host"
	TypeUnknown DataType = "unknown"
)

const stdLibWAT = `
;; --- Built-in Memory & String Library (Thread-Safe) ---
;; Heap Pointer is at offset 1020 (i32)
;; Shadow Stack Pointer is global per thread
(global $shadow_stack_base (mut i32) (i32.const 1024))
(global $shadow_stack_ptr (mut i32) (i32.const 1024))
(global $allocated_list (mut i32) (i32.const 0))

(func $set_stack_pointer (param $ptr i32)
  local.get $ptr
  global.set $shadow_stack_base
  local.get $ptr
  global.set $shadow_stack_ptr
)

(func $malloc (param $size i32) (param $type_id i32) (result i32)
  (local $ptr i32)
  (local $total i32)
  
  ;; Align size to 8 bytes
  local.get $size
  i32.const 7
  i32.add
  i32.const -8
  i32.and
  local.set $size

  ;; Header size: 16 bytes
  local.get $size
  i32.const 16
  i32.add
  local.set $total
  
  ;; Atomic bump pointer at 1020
  ;; ptr = atomic_add(1020, total)
  i32.const 1020
  local.get $total
  i32.atomic.rmw.add
  local.set $ptr
  
  ;; Initialize header
  ;; Offset 0: next_allocated (dummy 0 for now)
  local.get $ptr
  i32.const 0
  i32.store
  
  ;; Offset 4: size
  local.get $ptr
  i32.const 4
  i32.add
  local.get $size
  i32.store
  
  ;; Offset 8: mark (0)
  local.get $ptr
  i32.const 8
  i32.add
  i32.const 0
  i32.store
  
  ;; Offset 12: type_id
  local.get $ptr
  i32.const 12
  i32.add
  local.get $type_id
  i32.store
  
  ;; Return payload (ptr + 16)
  local.get $ptr
  i32.const 16
  i32.add
)

(func $gc_mark (param $ptr i32)
  (local $header i32)
  (local $type_id i32)
  
  local.get $ptr
  i32.eqz
  if
    return
  end
  
  ;; Check valid heap (>= 10240)
  local.get $ptr
  i32.const 10240
  i32.lt_u
  if
    return
  end
  
  ;; Get header
  local.get $ptr
  i32.const 16
  i32.sub
  local.set $header
  
  ;; Check mark
  local.get $header
  i32.const 8
  i32.add
  i32.load
  if
    return
  end
  
  ;; Mark
  local.get $header
  i32.const 8
  i32.add
  i32.const 1
  i32.store
  
  ;; Trace
  local.get $header
  i32.const 12
  i32.add
  i32.load
  local.set $type_id
  
  local.get $ptr
  local.get $type_id
  call $gc_trace
)

(func $gc_collect
  ;; No-op for concurrent MVP
)

(func $free (param $ptr i32)
  ;; No-op
)

(func $strlen (param $str i32) (result i32)
  (local $len i32)
  (local $ptr i32)
  local.get $str
  local.set $ptr
  (block $break
    (loop $top
      local.get $ptr
      i32.load8_u
      i32.eqz
      br_if $break
      local.get $len
      i32.const 1
      i32.add
      local.set $len
      local.get $ptr
      i32.const 1
      i32.add
      local.set $ptr
      br $top
    )
  )
  local.get $len
)

(func $str_concat (param $s1 i32) (param $s2 i32) (result i32)
  (local $len1 i32)
  (local $len2 i32)
  (local $new_ptr i32)
  (local $dest i32)
  (local $src i32)
  local.get $s1
  call $strlen
  local.set $len1
  local.get $s2
  call $strlen
  local.set $len2
  local.get $len1
  local.get $len2
  i32.add
  i32.const 1
  i32.add
  i32.const 0 ;; TypeID 0 (String)
  call $malloc
  local.set $new_ptr
  local.get $new_ptr
  local.set $dest
  local.get $s1
  local.set $src
  (block $b1 (loop $l1
     local.get $src
     i32.load8_u
     i32.eqz
     br_if $b1
     local.get $dest
     local.get $src
     i32.load8_u
     i32.store8
     local.get $dest
     i32.const 1
     i32.add
     local.set $dest
     local.get $src
     i32.const 1
     i32.add
     local.set $src
     br $l1
  ))
  local.get $s2
  local.set $src
  (block $b2 (loop $l2
     local.get $src
     i32.load8_u
     i32.eqz
     br_if $b2
     local.get $dest
     local.get $src
     i32.load8_u
     i32.store8
     local.get $dest
     i32.const 1
     i32.add
     local.set $dest
     local.get $src
     i32.const 1
     i32.add
     local.set $src
     br $l2
  ))
  local.get $dest
  i32.const 0
  i32.store8
  local.get $new_ptr
)

(func $string_substring (param $str i32) (param $start i32) (param $end i32) (result i32)
  (local $len i32)
  (local $new_ptr i32)
  (local $src i32)
  (local $dest i32)
  (local $i i32)

  ;; Calculate length = end - start
  local.get $end
  local.get $start
  i32.sub
  local.set $len
  
  ;; Allocate new string (len + 1 for null terminator)
  local.get $len
  i32.const 1
  i32.add
  i32.const 0 ;; TypeID 0 (String)
  call $malloc
  local.set $new_ptr
  
  ;; Copy bytes
  local.get $str
  local.get $start
  i32.add
  local.set $src
  
  local.get $new_ptr
  local.set $dest
  
  i32.const 0
  local.set $i
  
  (block $done_copy
    (loop $copy
      local.get $i
      local.get $len
      i32.ge_u
      br_if $done_copy
      
      local.get $dest
      local.get $i
      i32.add
      
      local.get $src
      local.get $i
      i32.add
      i32.load8_u
      
      i32.store8
      
      local.get $i
      i32.const 1
      i32.add
      local.set $i
      br $copy
    )
  )
  
  ;; Null terminate
  local.get $dest
  local.get $len
  i32.add
  i32.const 0
  i32.store8
  
  local.get $new_ptr
)

(func $string_indexOfChar (param $str i32) (param $char i32) (result i32)
  (local $len i32)
  (local $i i32)
  
  local.get $str
  call $strlen
  local.set $len
  
  i32.const 0
  local.set $i
  
  (block $done
    (loop $loop
      local.get $i
      local.get $len
      i32.ge_u
      br_if $done
      
      local.get $str
      local.get $i
      i32.add
      i32.load8_u
      local.get $char
      i32.eq
      if
        local.get $i
        return
      end
      
      local.get $i
      i32.const 1
      i32.add
      local.set $i
      br $loop
    )
  )
  
  i32.const -1
)

(func $array_new (param $cap i32) (result i32)
  (local $arr i32)
  (local $data i32)
  
  ;; Allocate array struct (12 bytes)
  i32.const 12
  i32.const 1 ;; TypeID 1
  call $malloc
  local.set $arr
  
  ;; Set length = 0
  local.get $arr
  i32.const 0
  i32.store
  
  ;; Set capacity
  local.get $arr
  i32.const 4
  i32.add
  local.get $cap
  i32.store
  
  ;; Allocate data buffer (cap * 4)
  local.get $cap
  i32.const 4
  i32.mul
  i32.const 20 ;; TypeID 20
  call $malloc
  local.set $data
  
  ;; Set data ptr
  local.get $arr
  i32.const 8
  i32.add
  local.get $data
  i32.store
  
  local.get $arr
)

(func $array_push (param $arr i32) (param $val i32)
  (local $len i32)
  (local $cap i32)
  (local $data i32)
  (local $new_cap i32)
  (local $new_data i32)
  (local $i i32)
  
  ;; Get length
  local.get $arr
  i32.load
  local.set $len
  
  ;; Get capacity
  local.get $arr
  i32.const 4
  i32.add
  i32.load
  local.set $cap
  
  ;; Get data ptr
  local.get $arr
  i32.const 8
  i32.add
  i32.load
  local.set $data
  
  ;; Check capacity
  local.get $len
  local.get $cap
  i32.ge_u
  if
    ;; Resize: double capacity
    local.get $cap
    i32.const 2
    i32.mul
    local.set $new_cap
    
    ;; Cap at least 4
    local.get $new_cap
    i32.const 4
    i32.lt_u
    if
      i32.const 4
      local.set $new_cap
    end
    
    ;; Allocate new data
    local.get $new_cap
    i32.const 4
    i32.mul
    i32.const 20 ;; TypeID 20
    call $malloc
    local.set $new_data
    
    ;; Copy old data
    i32.const 0
    local.set $i
    (block $done_copy
      (loop $copy
        local.get $i
        local.get $len
        i32.ge_u
        br_if $done_copy
        
        ;; new_data[i] = old_data[i]
        local.get $new_data
        local.get $i
        i32.const 4
        i32.mul
        i32.add
        
        local.get $data
        local.get $i
        i32.const 4
        i32.mul
        i32.add
        i32.load
        
        i32.store
        
        local.get $i
        i32.const 1
        i32.add
        local.set $i
        br $copy
      )
    )
    
    ;; Update array struct
    local.get $arr
    i32.const 4
    i32.add
    local.get $new_cap
    i32.store
    
    local.get $arr
    i32.const 8
    i32.add
    local.get $new_data
    i32.store
    
    ;; Update locals
    local.get $new_data
    local.set $data
  end
  
  ;; Store value
  local.get $data
  local.get $len
  i32.const 4
  i32.mul
  i32.add
  local.get $val
  i32.store
  
  ;; Increment length
  local.get $arr
  local.get $len
  i32.const 1
  i32.add
  i32.store
)

(func $array_get (param $arr i32) (param $idx i32) (result i32)
  (local $data i32)
  (local $len i32)
  
  ;; Get length
  local.get $arr
  i32.load
  local.set $len
  
  ;; Bounds check (TODO: Trap/Panic if out of bounds)
  local.get $idx
  local.get $len
  i32.ge_u
  if
    i32.const 0
    return
  end

  ;; Get data ptr
  local.get $arr
  i32.const 8
  i32.add
  i32.load
  local.set $data
  
  ;; Load value
  local.get $data
  local.get $idx
  i32.const 4
  i32.mul
  i32.add
  i32.load
)

(func $array_set (param $arr i32) (param $idx i32) (param $val i32)
  (local $data i32)
  (local $len i32)
  
  ;; Get length
  local.get $arr
  i32.load
  local.set $len
  
  ;; Bounds check
  local.get $idx
  local.get $len
  i32.ge_u
  if
    return
  end

  ;; Get data ptr
  local.get $arr
  i32.const 8
  i32.add
  i32.load
  local.set $data
  
  ;; Store value
  local.get $data
  local.get $idx
  i32.const 4
  i32.mul
  i32.add
  local.get $val
  i32.store
)

(func $array_length (param $arr i32) (result i32)
  local.get $arr
  i32.load
)

(func $string_equals (param $s1 i32) (param $s2 i32) (result i32)
  (local $len1 i32)
  (local $len2 i32)
  (local $i i32)
  
  local.get $s1
  call $strlen
  local.set $len1
  
  local.get $s2
  call $strlen
  local.set $len2
  
  local.get $len1
  local.get $len2
  i32.ne
  if
    i32.const 0
    return
  end
  
  i32.const 0
  local.set $i
  
  (block $done
    (loop $loop
      local.get $i
      local.get $len1
      i32.ge_u
      br_if $done
      
      local.get $s1
      local.get $i
      i32.add
      i32.load8_u
      
      local.get $s2
      local.get $i
      i32.add
      i32.load8_u
      
      i32.ne
      if
        i32.const 0
        return
      end
      
      local.get $i
      i32.const 1
      i32.add
      local.set $i
      br $loop
    )
  )
  i32.const 1
)

(func $hash_string (param $str i32) (result i32)
  ;; djb2 hash
  (local $hash i32)
  (local $c i32)
  (local $i i32)
  (local $len i32)
  
  i32.const 5381
  local.set $hash
  
  local.get $str
  call $strlen
  local.set $len
  
  i32.const 0
  local.set $i
  
  (block $done
    (loop $loop
      local.get $i
      local.get $len
      i32.ge_u
      br_if $done
      
      local.get $str
      local.get $i
      i32.add
      i32.load8_u
      local.set $c
      
      ;; hash = ((hash << 5) + hash) + c
      local.get $hash
      i32.const 5
      i32.shl
      local.get $hash
      i32.add
      local.get $c
      i32.add
      local.set $hash
      
      local.get $i
      i32.const 1
      i32.add
      local.set $i
      br $loop
    )
  )
  
  local.get $hash
)

(func $map_new (result i32)
  (local $map i32)
  (local $buckets i32)
  (local $i i32)
  
  ;; Allocate Map (12 bytes: capacity, count, buckets)
  i32.const 12
  i32.const 2 ;; TypeID 2 (Map)
  call $malloc
  local.set $map
  
  ;; Set capacity = 16
  local.get $map
  i32.const 16
  i32.store
  
  ;; Set count = 0
  local.get $map
  i32.const 4
  i32.add
  i32.const 0
  i32.store
  
  ;; Allocate buckets (16 * 4 bytes)
  i32.const 64
  i32.const 21 ;; TypeID 21 (MapBuckets)
  call $malloc
  local.set $buckets
  
  ;; Initialize buckets to 0
  i32.const 0
  local.set $i
  (block $done_zero
    (loop $zero
      local.get $i
      i32.const 64
      i32.ge_u
      br_if $done_zero
      
      local.get $buckets
      local.get $i
      i32.add
      i32.const 0
      i32.store
      
      local.get $i
      i32.const 4
      i32.add
      local.set $i
      br $zero
    )
  )
  
  ;; Set buckets ptr
  local.get $map
  i32.const 8
  i32.add
  local.get $buckets
  i32.store
  
  local.get $map
)

(func $map_set (param $map i32) (param $key i32) (param $val i32)
  (local $hash i32)
  (local $cap i32)
  (local $buckets i32)
  (local $idx i32)
  (local $entry i32)
  
  ;; Calculate hash
  local.get $key
  call $hash_string
  local.set $hash
  
  ;; Get capacity
  local.get $map
  i32.load
  local.set $cap
  
  ;; Get buckets
  local.get $map
  i32.const 8
  i32.add
  i32.load
  local.set $buckets
  
  ;; Index = hash % cap
  local.get $hash
  local.get $cap
  i32.const 1
  i32.sub
  i32.and
  local.set $idx
  
  ;; Walk linked list at buckets[idx]
  local.get $buckets
  local.get $idx
  i32.const 4
  i32.mul
  i32.add
  i32.load
  local.set $entry
  
  (block $found
    (loop $search
      local.get $entry
      i32.eqz
      br_if $found ;; Not found, create new
      
      ;; Check key
      local.get $entry
      i32.load ;; key is at offset 0
      local.get $key
      call $string_equals
      if
        ;; Found! Update value
        local.get $entry
        i32.const 4
        i32.add
        local.get $val
        i32.store
        return
      end
      
      local.get $entry
      i32.const 8
      i32.add
      i32.load ;; next is at offset 8
      local.set $entry
      br $search
    )
  )
  
  ;; Create new entry (12 bytes: key, value, next)
  i32.const 12
  i32.const 22 ;; TypeID 22 (MapEntry)
  call $malloc
  local.set $entry
  
  ;; Set key
  local.get $entry
  local.get $key
  i32.store
  
  ;; Set value
  local.get $entry
  i32.const 4
  i32.add
  local.get $val
  i32.store
  
  ;; Set next = buckets[idx]
  local.get $entry
  i32.const 8
  i32.add
  
  local.get $buckets
  local.get $idx
  i32.const 4
  i32.mul
  i32.add
  i32.load
  
  i32.store
  
  ;; Update buckets[idx] = entry
  local.get $buckets
  local.get $idx
  i32.const 4
  i32.mul
  i32.add
  local.get $entry
  i32.store
  
  ;; Increment count
  local.get $map
  i32.const 4
  i32.add
  
  local.get $map
  i32.const 4
  i32.add
  i32.load
  i32.const 1
  i32.add
  
  i32.store
)

(func $map_get (param $map i32) (param $key i32) (result i32)
  (local $hash i32)
  (local $cap i32)
  (local $buckets i32)
  (local $idx i32)
  (local $entry i32)
  
  ;; Calculate hash
  local.get $key
  call $hash_string
  local.set $hash
  
  ;; Get capacity
  local.get $map
  i32.load
  local.set $cap
  
  ;; Get buckets
  local.get $map
  i32.const 8
  i32.add
  i32.load
  local.set $buckets
  
  ;; Index
  local.get $hash
  local.get $cap
  i32.const 1
  i32.sub
  i32.and
  local.set $idx
  
  ;; Walk
  local.get $buckets
  local.get $idx
  i32.const 4
  i32.mul
  i32.add
  i32.load
  local.set $entry
  
  (block $not_found
    (loop $search
      local.get $entry
      i32.eqz
      br_if $not_found
      
      local.get $entry
      i32.load
      local.get $key
      call $string_equals
      if
        ;; Found
        local.get $entry
        i32.const 4
        i32.add
        i32.load
        return
      end
      
      local.get $entry
      i32.const 8
      i32.add
      i32.load
      local.set $entry
      br $search
    )
  )
  
  ;; Not found
  i32.const 0
)

(func $itos (param $val i32) (result i32)
  (local $ptr i32)
  (local $len i32)
  (local $temp i32)
  (local $is_neg i32)
  (local $end i32)
  (local $start i32)
  
  ;; Handle 0 specially
  local.get $val
  i32.eqz
  if
    i32.const 2
    i32.const 0
    call $malloc
    local.set $ptr
    
    local.get $ptr
    i32.const 48 ;; '0'
    i32.store8
    
    local.get $ptr
    i32.const 1
    i32.add
    i32.const 0
    i32.store8
    
    local.get $ptr
    return
  end
  
  ;; Check negative
  local.get $val
  i32.const 0
  i32.lt_s
  if
    i32.const 1
    local.set $is_neg
    i32.const 0
    local.get $val
    i32.sub
    local.set $val
  end
  
  ;; Count digits
  local.get $val
  local.set $temp
  i32.const 0
  local.set $len
  
  (block $count_done
    (loop $count_loop
      local.get $temp
      i32.eqz
      br_if $count_done
      
      local.get $len
      i32.const 1
      i32.add
      local.set $len
      
      local.get $temp
      i32.const 10
      i32.div_u
      local.set $temp
      br $count_loop
    )
  )
  
  ;; Add 1 for sign if negative
  local.get $is_neg
  if
    local.get $len
    i32.const 1
    i32.add
    local.set $len
  end
  
  ;; Allocate string (len + 1)
  local.get $len
  i32.const 1
  i32.add
  i32.const 0 ;; TypeID 0
  call $malloc
  local.set $ptr
  
  ;; Null terminate
  local.get $ptr
  local.get $len
  i32.add
  i32.const 0
  i32.store8
  
  ;; Fill digits backwards
  local.get $ptr
  local.get $len
  i32.add
  i32.const 1
  i32.sub
  local.set $end
  
  local.get $ptr
  local.set $start
  
  local.get $is_neg
  if
    local.get $ptr
    i32.const 45 ;; '-'
    i32.store8
    
    local.get $start
    i32.const 1
    i32.add
    local.set $start
  end
  
  (block $fill_done
    (loop $fill_loop
      local.get $end
      local.get $start
      i32.lt_u
      br_if $fill_done
      
      local.get $val
      i32.const 10
      i32.rem_u
      local.set $temp
      
      local.get $end
      local.get $temp
      i32.const 48 ;; '0'
      i32.add
      i32.store8
      
      local.get $val
      i32.const 10
      i32.div_u
      local.set $val
      
      local.get $end
      i32.const 1
      i32.sub
      local.set $end
      br $fill_loop
    )
  )
  
  local.get $ptr
)

(func $string_lastIndexOfChar (param $str i32) (param $char i32) (result i32)
  (local $len i32)
  (local $i i32)
  
  local.get $str
  call $strlen
  local.set $len
  
  local.get $len
  i32.eqz
  if
    i32.const -1
    return
  end
  
  local.get $len
  local.set $i
  
  (block $done
    (loop $loop
      local.get $i
      i32.const 0
      i32.eq
      br_if $done
      
      local.get $i
      i32.const 1
      i32.sub
      local.set $i
      
      local.get $str
      local.get $i
      i32.add
      i32.load8_u
      local.get $char
      i32.eq
      if
        local.get $i
        return
      end
      
      br $loop
    )
  )
  
  i32.const -1
)

(func $path_basename (param $path i32) (result i32)
  (local $idx i32)
  (local $len i32)
  
  local.get $path
  i32.const 47 ;; '/'
  call $string_lastIndexOfChar
  local.set $idx
  
  local.get $idx
  i32.const -1
  i32.eq
  if
    local.get $path
    return
  end
  
  local.get $path
  call $strlen
  local.set $len
  
  local.get $path
  local.get $idx
  i32.const 1
  i32.add
  local.get $len
  call $string_substring
)

(func $path_dirname (param $path i32) (result i32)
  (local $idx i32)
  (local $sep i32)
  
  local.get $path
  i32.const 47 ;; '/'
  call $string_lastIndexOfChar
  local.set $idx
  
  local.get $idx
  i32.const -1
  i32.eq
  if
    ;; "."
    i32.const 2
    i32.const 0
    call $malloc
    local.set $sep
    local.get $sep
    i32.const 46 ;; '.'
    i32.store8
    local.get $sep
    i32.const 1
    i32.add
    i32.const 0
    i32.store8
    local.get $sep
    return
  end
  
  local.get $idx
  i32.const 0
  i32.eq
  if
    ;; "/"
    i32.const 2
    i32.const 0
    call $malloc
    local.set $sep
    local.get $sep
    i32.const 47 ;; '/'
    i32.store8
    local.get $sep
    i32.const 1
    i32.add
    i32.const 0
    i32.store8
    local.get $sep
    return
  end
  
  local.get $path
  i32.const 0
  local.get $idx
  call $string_substring
)

(func $path_extname (param $path i32) (result i32)
  (local $idx i32)
  (local $len i32)
  
  local.get $path
  i32.const 46 ;; '.'
  call $string_lastIndexOfChar
  local.set $idx
  
  local.get $idx
  i32.const -1
  i32.eq
  if
    ;; ""
    i32.const 1
    i32.const 0
    call $malloc
    local.set $path
    local.get $path
    i32.const 0
    i32.store8
    local.get $path
    return
  end
  
  local.get $idx
  i32.const 0
  i32.eq
  if
    ;; "" (hidden file)
    i32.const 1
    i32.const 0
    call $malloc
    local.set $path
    local.get $path
    i32.const 0
    i32.store8
    local.get $path
    return
  end

  local.get $path
  call $strlen
  local.set $len
  
  local.get $path
  local.get $idx
  local.get $len
  call $string_substring
)

(func $path_join2 (param $p1 i32) (param $p2 i32) (result i32)
  (local $len1 i32)
  (local $len2 i32)
  (local $sep i32)
  
  local.get $p1
  call $strlen
  local.set $len1
  
  local.get $p2
  call $strlen
  local.set $len2
  
  local.get $len1
  i32.eqz
  if
    local.get $p2
    return
  end
  
  local.get $len2
  i32.eqz
  if
    local.get $p1
    return
  end
  
  ;; Check last char of p1
  local.get $p1
  local.get $len1
  i32.const 1
  i32.sub
  i32.add
  i32.load8_u
  i32.const 47 ;; '/'
  i32.eq
  if
    ;; p1 ends with /, just concat
    local.get $p1
    local.get $p2
    call $str_concat
    return
  end
  
  ;; Concat p1 + "/" + p2
  i32.const 2
  i32.const 0
  call $malloc
  local.set $sep
  local.get $sep
  i32.const 47
  i32.store8
  local.get $sep
  i32.const 1
  i32.add
  i32.const 0
  i32.store8
  
  local.get $p1
  local.get $sep
  call $str_concat
  local.get $p2
  call $str_concat
)

(func $process_env (result i32)
  (local $count i32)
  (local $buf_size i32)
  (local $environ i32)
  (local $environ_buf i32)
  (local $map i32)
  (local $i i32)
  (local $env_ptr i32)
  (local $len i32)
  (local $eq_idx i32)
  (local $key i32)
  (local $val i32)
  (local $count_ptr i32)
  (local $buf_size_ptr i32)

  ;; Allocate space for count and buf_size
  i32.const 4
  i32.const 0
  call $malloc
  local.set $count_ptr
  
  i32.const 4
  i32.const 0
  call $malloc
  local.set $buf_size_ptr

  ;; Get sizes
  local.get $count_ptr
  local.get $buf_size_ptr
  call $environ_sizes_get
  drop

  local.get $count_ptr
  i32.load
  local.set $count
  
  local.get $buf_size_ptr
  i32.load
  local.set $buf_size

  ;; Allocate environ (array of pointers)
  local.get $count
  i32.const 4
  i32.mul
  i32.const 0
  call $malloc
  local.set $environ

  ;; Allocate environ_buf (char buffer)
  local.get $buf_size
  i32.const 0
  call $malloc
  local.set $environ_buf

  ;; Get environ
  local.get $environ
  local.get $environ_buf
  call $environ_get
  drop

  ;; Create Map
  call $map_new
  local.set $map

  ;; Iterate
  i32.const 0
  local.set $i

  (block $done
    (loop $loop
      local.get $i
      local.get $count
      i32.ge_u
      br_if $done

      ;; env_ptr = environ[i]
      local.get $environ
      local.get $i
      i32.const 4
      i32.mul
      i32.add
      i32.load
      local.set $env_ptr
      
      ;; len = strlen(env_ptr)
      local.get $env_ptr
      call $strlen
      local.set $len
      
      ;; eq_idx = string_indexOfChar(env_ptr, 61) ;; '=' is 61
      local.get $env_ptr
      i32.const 61
      call $string_indexOfChar
      local.set $eq_idx
      
      ;; if eq_idx != -1
      local.get $eq_idx
      i32.const -1
      i32.ne
      if
        ;; key = substring(env_ptr, 0, eq_idx)
        local.get $env_ptr
        i32.const 0
        local.get $eq_idx
        call $string_substring
        local.set $key
        
        ;; val = substring(env_ptr, eq_idx + 1, len)
        local.get $env_ptr
        local.get $eq_idx
        i32.const 1
        i32.add
        local.get $len
        call $string_substring
        local.set $val
        
        ;; map_set(map, key, val)
        local.get $map
        local.get $key
        local.get $val
        call $map_set
      end

      local.get $i
      i32.const 1
      i32.add
      local.set $i
      br $loop
    )
  )

  local.get $map
)
`


/* (func $string_charCodeAt (param $str i32) (param $index i32) (result i32)
  ;; Load byte at str + index
  local.get $str
  local.get $index
  i32.add
  i32.load8_u
)

(func $array_new (param $capacity i32) (result i32)
  (local $arr i32)
  (local $data i32)
  
  ;; Allocate Array struct (12 bytes: len, cap, data)
  i32.const 12
  i32.const 1 ;; TypeID 1 (Array)
  call $malloc
  local.set $arr
  
  ;; Set length = 0
  local.get $arr
  i32.const 0
  i32.store
  
  ;; Set capacity
  local.get $arr
  i32.const 4
  i32.add
  local.get $capacity
  i32.store
  
  ;; Allocate data buffer
  local.get $capacity
  i32.const 4
  i32.mul
  i32.const 20 ;; TypeID 20 (ArrayData)
  call $malloc
  local.set $data
  
  ;; Set data pointer
  local.get $arr
  i32.const 8
  i32.add
  local.get $data
  i32.store
  
  local.get $arr
)

(func $array_push (param $arr i32) (param $val i32)
  (local $len i32)
  (local $cap i32)
  (local $data i32)
  (local $new_cap i32)
  (local $new_data i32)
  (local $i i32)
  
  ;; Get length
  local.get $arr
  i32.load
  local.set $len
  
  ;; Get capacity
  local.get $arr
  i32.const 4
  i32.add
  i32.load
  local.set $cap
  
  ;; Get data ptr
  local.get $arr
  i32.const 8
  i32.add
  i32.load
  local.set $data
  
  ;; Check capacity
  local.get $len
  local.get $cap
  i32.ge_u
  if
    ;; Resize: double capacity
    local.get $cap
    i32.const 2
    i32.mul
    local.set $new_cap
    
    ;; Cap at least 4
    local.get $new_cap
    i32.const 4
    i32.lt_u
    if
      i32.const 4
      local.set $new_cap
    end
    
    ;; Allocate new data
    local.get $new_cap
    i32.const 4
    i32.mul
    i32.const 20 ;; TypeID 20
    call $malloc
    local.set $new_data
    
    ;; Copy old data
    i32.const 0
    local.set $i
    (block $done_copy
      (loop $copy
        local.get $i
        local.get $len
        i32.ge_u
        br_if $done_copy
        
        ;; new_data[i] = old_data[i]
        local.get $new_data
        local.get $i
        i32.const 4
        i32.mul
        i32.add
        
        local.get $data
        local.get $i
        i32.const 4
        i32.mul
        i32.add
        i32.load
        
        i32.store
        
        local.get $i
        i32.const 1
        i32.add
        local.set $i
        br $copy
      )
    )
    
    ;; Update array struct
    local.get $arr
    i32.const 4
    i32.add
    local.get $new_cap
    i32.store
    
    local.get $arr
    i32.const 8
    i32.add
    local.get $new_data
    i32.store
    
    ;; Update locals
    local.get $new_data
    local.set $data
  end
  
  ;; Store value
  local.get $data
  local.get $len
  i32.const 4
  i32.mul
  i32.add
  local.get $val
  i32.store
  
  ;; Increment length
  local.get $arr
  local.get $len
  i32.const 1
  i32.add
  i32.store
)

(func $array_get (param $arr i32) (param $idx i32) (result i32)
  (local $data i32)
  (local $len i32)
  
  ;; Get length
  local.get $arr
  i32.load
  local.set $len
  
  ;; Bounds check (TODO: Trap/Panic if out of bounds)
  local.get $idx
  local.get $len
  i32.ge_u
  if
    i32.const 0
    return
  end

  ;; Get data ptr
  local.get $arr
  i32.const 8
  i32.add
  i32.load
  local.set $data
  
  ;; Load value
  local.get $data
  local.get $idx
  i32.const 4
  i32.mul
  i32.add
  i32.load
)

(func $array_set (param $arr i32) (param $idx i32) (param $val i32)
  (local $data i32)
  (local $len i32)
  
  ;; Get length
  local.get $arr
  i32.load
  local.set $len
  
  ;; Bounds check
  local.get $idx
  local.get $len
  i32.ge_u
  if
    return
  end

  ;; Get data ptr
  local.get $arr
  i32.const 8
  i32.add
  i32.load
  local.set $data
  
  ;; Store value
  local.get $data
  local.get $idx
  i32.const 4
  i32.mul
  i32.add
  local.get $val
  i32.store
)

(func $array_length (param $arr i32) (result i32)
  local.get $arr
  i32.load
)

(func $string_equals (param $s1 i32) (param $s2 i32) (result i32)
  (local $len1 i32)
  (local $len2 i32)
  (local $i i32)
  
  local.get $s1
  call $strlen
  local.set $len1
  
  local.get $s2
  call $strlen
  local.set $len2
  
  local.get $len1
  local.get $len2
  i32.ne
  if
    i32.const 0
    return
  end
  
  i32.const 0
  local.set $i
  
  (block $done
    (loop $loop
      local.get $i
      local.get $len1
      i32.ge_u
      br_if $done
      
      local.get $s1
      local.get $i
      i32.add
      i32.load8_u
      
      local.get $s2
      local.get $i
      i32.add
      i32.load8_u
      
      i32.ne
      if
        i32.const 0
        return
      end
      
      local.get $i
      i32.const 1
      i32.add
      local.set $i
      br $loop
    )
  )
  i32.const 1
)

(func $hash_string (param $str i32) (result i32)
  ;; djb2 hash
  (local $hash i32)
  (local $c i32)
  (local $i i32)
  (local $len i32)
  
  i32.const 5381
  local.set $hash
  
  local.get $str
  call $strlen
  local.set $len
  
  i32.const 0
  local.set $i
  
  (block $done
    (loop $loop
      local.get $i
      local.get $len
      i32.ge_u
      br_if $done
      
      local.get $str
      local.get $i
      i32.add
      i32.load8_u
      local.set $c
      
      ;; hash = ((hash << 5) + hash) + c
      local.get $hash
      i32.const 5
      i32.shl
      local.get $hash
      i32.add
      local.get $c
      i32.add
      local.set $hash
      
      local.get $i
      i32.const 1
      i32.add
      local.set $i
      br $loop
    )
  )
  
  local.get $hash
)

(func $map_new (result i32)
  (local $map i32)
  (local $buckets i32)
  (local $i i32)
  
  ;; Allocate Map (12 bytes: capacity, count, buckets)
  i32.const 12
  i32.const 2 ;; TypeID 2 (Map)
  call $malloc
  local.set $map
  
  ;; Set capacity = 16
  local.get $map
  i32.const 16
  i32.store
  
  ;; Set count = 0
  local.get $map
  i32.const 4
  i32.add
  i32.const 0
  i32.store
  
  ;; Allocate buckets (16 * 4 bytes)
  i32.const 64
  i32.const 21 ;; TypeID 21 (MapBuckets)
  call $malloc
  local.set $buckets
  
  ;; Initialize buckets to 0 (malloc might not zero?)
  ;; Actually, malloc reuses memory, so we MUST zero.
  ;; For now, assume we implement memset or loop.
  ;; Let's zero it.
  i32.const 0
  local.set $i
  (block $done_zero
    (loop $zero
      local.get $i
      i32.const 64
      i32.ge_u
      br_if $done_zero
      
      local.get $buckets
      local.get $i
      i32.add
      i32.const 0
      i32.store
      
      local.get $i
      i32.const 4
      i32.add
      local.set $i
      br $zero
    )
  )
  
  ;; Set buckets ptr
  local.get $map
  i32.const 8
  i32.add
  local.get $buckets
  i32.store
  
  local.get $map
)

(func $map_set (param $map i32) (param $key i32) (param $val i32)
  (local $hash i32)
  (local $cap i32)
  (local $buckets i32)
  (local $idx i32)
  (local $entry i32)
  (local $prev i32)
  
  ;; Calculate hash
  local.get $key
  call $hash_string
  local.set $hash
  
  ;; Get capacity
  local.get $map
  i32.load
  local.set $cap
  
  ;; Get buckets
  local.get $map
  i32.const 8
  i32.add
  i32.load
  local.set $buckets
  
  ;; Index = hash % cap
  ;; Since cap is power of 2 (16), hash & (cap-1)
  local.get $hash
  local.get $cap
  i32.const 1
  i32.sub
  i32.and
  local.set $idx
  
  ;; Walk linked list at buckets[idx]
  local.get $buckets
  local.get $idx
  i32.const 4
  i32.mul
  i32.add
  i32.load
  local.set $entry
  
  (block $found
    (loop $search
      local.get $entry
      i32.eqz
      br_if $found ;; Not found, create new
      
      ;; Check key
      local.get $entry
      i32.load ;; key is at offset 0
      local.get $key
      call $string_equals
      if
        ;; Found! Update value
        local.get $entry
        i32.const 4
        i32.add
        local.get $val
        i32.store
        return
      end
      
      local.get $entry
      i32.const 8
      i32.add
      i32.load ;; next is at offset 8
      local.set $entry
      br $search
    )
  )
  
  ;; Create new entry (12 bytes: key, value, next)
  i32.const 12
  i32.const 22 ;; TypeID 22 (MapEntry)
  call $malloc
  local.set $entry
  
  ;; Set key
  local.get $entry
  local.get $key
  i32.store
  
  ;; Set value
  local.get $entry
  i32.const 4
  i32.add
  local.get $val
  i32.store
  
  ;; Set next = buckets[idx]
  local.get $entry
  i32.const 8
  i32.add
  
  local.get $buckets
  local.get $idx
  i32.const 4
  i32.mul
  i32.add
  i32.load
  
  i32.store
  
  ;; Update buckets[idx] = entry
  local.get $buckets
  local.get $idx
  i32.const 4
  i32.mul
  i32.add
  local.get $entry
  i32.store
  
  ;; Increment count
  local.get $map
  i32.const 4
  i32.add
  
  local.get $map
  i32.const 4
  i32.add
  i32.load
  i32.const 1
  i32.add
  
  i32.store
  
  ;; TODO: Resize if load factor too high
)

(func $map_get (param $map i32) (param $key i32) (result i32)
  (local $hash i32)
  (local $cap i32)
  (local $buckets i32)
  (local $idx i32)
  (local $entry i32)
  
  ;; Calculate hash
  local.get $key
  call $hash_string
  local.set $hash
  
  ;; Get capacity
  local.get $map
  i32.load
  local.set $cap
  
  ;; Get buckets
  local.get $map
  i32.const 8
  i32.add
  i32.load
  local.set $buckets
  
  ;; Index
  local.get $hash
  local.get $cap
  i32.const 1
  i32.sub
  i32.and
  local.set $idx
  
  ;; Walk
  local.get $buckets
  local.get $idx
  i32.const 4
  i32.mul
  i32.add
  i32.load
  local.set $entry
  
  (block $not_found
    (loop $search
      local.get $entry
      i32.eqz
      br_if $not_found
      
      local.get $entry
      i32.load
      local.get $key
      call $string_equals
      if
        ;; Found
        local.get $entry
        i32.const 4
        i32.add
        i32.load
        return
      end
      
      local.get $entry
      i32.const 8
      i32.add
      i32.load
      local.set $entry
      br $search
    )
  )
  
  ;; Not found
  i32.const 0
)

(func $int_to_string (param $val i32) (result i32)
  (local $ptr i32)
  (local $len i32)
  (local $temp i32)
  (local $is_neg i32)
  (local $end i32)
  (local $start i32)
  
  ;; Handle 0 specially
  local.get $val
  i32.eqz
  if
    i32.const 2
    i32.const 0
    call $malloc
    local.set $ptr
    
    local.get $ptr
    i32.const 48 ;; '0'
    i32.store8
    
    local.get $ptr
    i32.const 1
    i32.add
    i32.const 0
    i32.store8
    
    local.get $ptr
    return
  end
  
  ;; Check negative
  local.get $val
  i32.const 0
  i32.lt_s
  if
    i32.const 1
    local.set $is_neg
    i32.const 0
    local.get $val
    i32.sub
    local.set $val
  end
  
  ;; Count digits
  local.get $val
  local.set $temp
  i32.const 0
  local.set $len
  
  (block $count_done
    (loop $count_loop
      local.get $temp
      i32.eqz
      br_if $count_done
      
      local.get $len
      i32.const 1
      i32.add
      local.set $len
      
      local.get $temp
      i32.const 10
      i32.div_u
      local.set $temp
      br $count_loop
    )
  )
  
  ;; Add 1 for sign if negative
  local.get $is_neg
  if
    local.get $len
    i32.const 1
    i32.add
    local.set $len
  end
  
  ;; Allocate string (len + 1)
  local.get $len
  i32.const 1
  i32.add
  i32.const 0 ;; TypeID 0
  call $malloc
  local.set $ptr
  
  ;; Null terminate
  local.get $ptr
  local.get $len
  i32.add
  i32.const 0
  i32.store8
  
  ;; Fill digits backwards
  local.get $ptr
  local.get $len
  i32.add
  i32.const 1
  i32.sub
  local.set $end
  
  local.get $ptr
  local.set $start
  
  local.get $is_neg
  if
    local.get $ptr
    i32.const 45 ;; '-'
    i32.store8
    
    local.get $start
    i32.const 1
    i32.add
    local.set $start
  end
  
  (block $fill_done
    (loop $fill_loop
      local.get $end
      local.get $start
      i32.lt_u
      br_if $fill_done
      
      local.get $val
      i32.const 10
      i32.rem_u
      local.set $temp
      
      local.get $end
      local.get $temp
      i32.const 48 ;; '0'
      i32.add
      i32.store8
      
      local.get $val
      i32.const 10
      i32.div_u
      local.set $val
      
      local.get $end
      i32.const 1
      i32.sub
      local.set $end
      br $fill_loop
    )
  )
  
  local.get $ptr
)
*/

type Symbol struct {
	Index   int
	Type    DataType
	IsParam bool
	ShadowIndex int // Index in the shadow stack (-1 if not tracked)
}

// FunctionScope represents a function being compiled
type FunctionScope struct {
	Name         string
	Instructions []string
	Symbols      map[string]Symbol
	NextLocalID  int
	ParamCount   int
	ParamTypes   []DataType
	ShadowStackSize int // Number of pointer locals tracked
}

type ClassSymbol struct {
	Name       string
	Size       int
	Fields     map[string]int      // Name -> Offset
	FieldTypes map[string]DataType // Name -> Type
	Methods    map[string]string   // Name -> MangledName
	Parent     string              // Parent class name (empty if none)
	TypeID     int                 // Unique Type ID for GC
}

type InterfaceSymbol struct {
	Name    string
	Methods map[string]*ast.MethodSignature
}

func NewFunctionScope(name string) *FunctionScope {
	return &FunctionScope{
		Name:         name,
		Instructions: []string{},
		Symbols:      make(map[string]Symbol),
		NextLocalID:  0,
		ParamCount:   0,
		ParamTypes:   []DataType{},
	}
}

type FunctionSignature struct {
	ParamTypes []DataType
}

// Compiler converts AST to WAT (WebAssembly Text Format)
type Compiler struct {
	functions      []*FunctionScope
	current        *FunctionScope
	imports        []string
	stringPool     map[string]int // String literal -> memory offset
	nextDataOffset int            // Next available memory offset
	classes        map[string]ClassSymbol // Class name -> Symbol
	interfaces     map[string]InterfaceSymbol // Interface name -> Symbol
	currentClass   string                 // Current class being compiled
	nextTypeID     int                    // Next available Type ID (start from 2)
	importedFuncs  map[string]*ast.ImportStatement // Imported functions
	definedFuncs   map[string]FunctionSignature    // Functions defined in source
	enums          map[string]map[string]int       // Enum definitions
	typeAliases    map[string]string               // Type aliases (Name -> TargetType)

	// Type checking state
	stackType DataType
	
	// Target platform ("wasi" or "browser")
	target string
}

func New(target string) *Compiler {
	c := &Compiler{
		functions:      []*FunctionScope{},
		imports:        []string{},
		stringPool:     make(map[string]int),
		nextDataOffset: 0,
		classes:        make(map[string]ClassSymbol),
		interfaces:     make(map[string]InterfaceSymbol),
		nextTypeID:     10, // 0-9 reserved. 0=String, 1=Array, 2=Map, 10+=Classes
		importedFuncs:  make(map[string]*ast.ImportStatement),
		definedFuncs:   make(map[string]FunctionSignature),
		enums:          make(map[string]map[string]int),
		typeAliases:    make(map[string]string),
		target:         target,
	}
	
	// Add Host Interop Imports
	c.imports = append(c.imports, `(import "env" "host_get_global" (func $host_get_global (param i32) (result i32)))`)
	c.imports = append(c.imports, `(import "env" "host_get" (func $host_get (param i32) (param i32) (result i32)))`)
	c.imports = append(c.imports, `(import "env" "host_set" (func $host_set (param i32) (param i32) (param i32)))`)
	c.imports = append(c.imports, `(import "env" "host_call" (func $host_call (param i32) (param i32) (param i32) (param i32) (result i32)))`)
	c.imports = append(c.imports, `(import "env" "host_from_int" (func $host_from_int (param i32) (result i32)))`)
	c.imports = append(c.imports, `(import "env" "host_from_string" (func $host_from_string (param i32) (result i32)))`)
	c.imports = append(c.imports, `(import "env" "host_to_int" (func $host_to_int (param i32) (result i32)))`)
	c.imports = append(c.imports, `(import "env" "thread_spawn" (func $thread_spawn (param i32 i32) (result i32)))`)
	
	return c
}

func (c *Compiler) resolveType(typeName string) DataType {
	visited := make(map[string]bool)
	current := typeName
	for {
		if visited[current] {
			return TypeUnknown // Cycle detected
		}
		visited[current] = true

		switch current {
		case "int":
			return TypeInt
		case "string":
			return TypeString
		case "bool":
			return TypeBool
		case "void":
			return TypeVoid
		case "array":
			return TypeArray
		case "map":
			return TypeMap
		}

		if strings.HasPrefix(current, "Array<") {
			return TypeArray
		}
		if strings.HasPrefix(current, "Map<") {
			return TypeMap
		}

		if alias, ok := c.typeAliases[current]; ok {
			current = alias
			continue
		}

		// Not a basic type or alias, assume Class/Interface (pointer) -> TypeInt
		return TypeInt
	}
}

func (c *Compiler) Compile(node ast.Node) error {
	switch node := node.(type) {
	case *ast.Program:
		// 1. First pass: Compile imports
		for _, stmt := range node.Statements {
			if _, ok := stmt.(*ast.ImportStatement); ok {
				if err := c.Compile(stmt); err != nil {
					return err
				}
			}
		}

		// 1.2 Pass: Define Type Aliases
		for _, stmt := range node.Statements {
			if typeAliasStmt, ok := stmt.(*ast.TypeAliasStatement); ok {
				c.typeAliases[typeAliasStmt.Name.Value] = typeAliasStmt.Value
			}
		}

		// 1.3 Pass: Define Enums
		for _, stmt := range node.Statements {
			if enumStmt, ok := stmt.(*ast.EnumStatement); ok {
				if err := c.defineEnum(enumStmt); err != nil {
					return err
				}
			}
		}

		// 1.4 Pass: Define Interfaces
		for _, stmt := range node.Statements {
			if ifaceStmt, ok := stmt.(*ast.InterfaceStatement); ok {
				if err := c.defineInterface(ifaceStmt); err != nil {
					return err
				}
			}
		}

		// 1.5 Pass: Define Classes
		for _, stmt := range node.Statements {
			if classStmt, ok := stmt.(*ast.ClassStatement); ok {
				if err := c.defineClass(classStmt); err != nil {
					return err
				}
			}
		}

		// 1.6 Pass: Compile Class Methods
		for _, stmt := range node.Statements {
			if classStmt, ok := stmt.(*ast.ClassStatement); ok {
				if err := c.compileClassMethods(classStmt); err != nil {
					return err
				}
			}
		}

		// 1.7 Pass: Check Interface Implementation
		for _, stmt := range node.Statements {
			if classStmt, ok := stmt.(*ast.ClassStatement); ok {
				if err := c.checkInterfaceImplementation(classStmt); err != nil {
					return err
				}
			}
		}

		// 1.8 Pass: Collect Function Names and Signatures
		for _, stmt := range node.Statements {
			if exprStmt, ok := stmt.(*ast.ExpressionStatement); ok {
				if fn, ok := exprStmt.Expression.(*ast.FunctionLiteral); ok {
					sig := FunctionSignature{ParamTypes: []DataType{}}
					for _, p := range fn.Parameters {
						t := c.resolveType(p.Type)
						sig.ParamTypes = append(sig.ParamTypes, t)
					}
					c.definedFuncs[fn.Name] = sig
				}
			}
		}

		// 2. Second pass: Compile functions
		for _, stmt := range node.Statements {
			if exprStmt, ok := stmt.(*ast.ExpressionStatement); ok {
				if fn, ok := exprStmt.Expression.(*ast.FunctionLiteral); ok {
					if err := c.compileFunction(fn); err != nil {
						return err
					}
				}
			}
		}

	case *ast.ImportStatement:
		// Generate Wasm import
		funcName := node.Name.Value
		
		paramsStr := ""
		for range node.Parameters {
			paramsStr += " (param i32)"
		}

		resultStr := ""
		if node.ReturnType != "void" && node.ReturnType != "" {
			resultStr = " (result i32)"
		}

		importStr := fmt.Sprintf("(import \"env\" \"%s\" (func $%s%s%s))", funcName, funcName, paramsStr, resultStr)
		c.imports = append(c.imports, importStr)
		c.importedFuncs[funcName] = node
	
	case *ast.InterfaceStatement:
		// Interfaces are compile-time only, no code generation needed.
		return nil

	case *ast.EnumStatement:
		// Enums are compile-time definitions (constants)
		return nil

	case *ast.SpawnStatement:
		// spawn func(args)
		// We need to:
		// 1. Get function index (table index) - MVP: pass function name/ID?
		// WASM `call_indirect` uses table index. We need to put functions in a table.
		// For MVP, we can't easily get table index dynamically without extensive changes.
		// Alternative: Pass function Name (string) to host, and host calls back?
		// But host can't call internal WASM functions easily by name unless exported.
		
		// Strategy:
		// 1. Compile call arguments into an Array or struct.
		// 2. Call $thread_spawn(func_id, args_ptr).
		// We need to map functions to IDs.
		
		// For MVP: Only support spawning 'main' or exported functions?
		// Or assume we export everything or use a table.
		
		// Let's use a simplified approach:
		// 1. Compile arguments -> store in array/struct.
		// 2. Identify the function being called.
		// 3. We need a dispatch function `(func $dispatch (param $id i32) (param $args i32))` exported to host.
		// 4. Host calls `dispatch` in new thread.
		
		// But adding a dispatch function requires knowing all possible spawn targets.
		// Let's assume for now we only spawn `function` calls (not methods).
		
		callExpr := node.Call
		var funcName string
		if ident, ok := callExpr.Function.(*ast.Identifier); ok {
			funcName = ident.Value
		} else {
			return fmt.Errorf("spawn only supports direct function calls")
		}
		
		// Create a unique ID for this function name (or use string)
		// Using string is easier for host to debug, but slower.
		// Let's use string for MVP.
		
		// 1. Pack arguments
		argCount := len(callExpr.Arguments)
		c.emit(fmt.Sprintf("i32.const %d", argCount))
		c.emit("call $array_new")
		// Stack: [arr]
		
		// Temp local for array
		arrTemp := c.current.NextLocalID
		c.current.NextLocalID++
		realArrTemp := arrTemp + c.current.ParamCount
		c.current.Symbols[fmt.Sprintf("$spawn_arr_%d", arrTemp)] = Symbol{Index: arrTemp, Type: TypeArray, IsParam: false, ShadowIndex: c.current.ShadowStackSize}
		c.current.ShadowStackSize++ // Track it
		
		c.emit(fmt.Sprintf("local.set %d", realArrTemp))
		
		// Push to shadow stack
		c.emit("global.get $shadow_stack_ptr")
		c.emit(fmt.Sprintf("local.get %d", realArrTemp))
		c.emit("i32.store")
		c.emit("global.get $shadow_stack_ptr")
		c.emit("i32.const 4")
		c.emit("i32.add")
		c.emit("global.set $shadow_stack_ptr")
		
		for _, arg := range callExpr.Arguments {
			c.emit(fmt.Sprintf("local.get %d", realArrTemp))
			if err := c.Compile(arg); err != nil {
				return err
			}
			c.emit("call $array_push")
		}
		
		// 2. Function Name
		offset, ok := c.stringPool[funcName]
		if !ok {
			offset = c.nextDataOffset
			c.stringPool[funcName] = offset
			c.nextDataOffset += len(funcName) + 1
		}
		c.emit(fmt.Sprintf("i32.const %d ;; \"%s\"", offset, funcName))
		
		// 3. Call thread_spawn(func_name_ptr, args_arr_ptr)
		c.emit(fmt.Sprintf("local.get %d", realArrTemp))
		c.emit("call $thread_spawn")
		c.emit("drop") // thread_spawn returns handle/id, ignore for now
		
		c.stackType = TypeVoid
		return nil

	case *ast.TypeAliasStatement:
		// Already handled in pass 1.2
		return nil

	case *ast.ExpressionStatement:
		if err := c.Compile(node.Expression); err != nil {
			return err
		}
		if c.stackType != TypeVoid {
			c.emit("drop")
		}

	case *ast.LetStatement:
		if err := c.Compile(node.Value); err != nil {
			return err
		}

		valueType := c.stackType
		if valueType == TypeUnknown {
			valueType = TypeInt // Default to int for MVP if unknown
		}

		index := c.current.NextLocalID
		shadowIndex := c.current.ShadowStackSize
		
		c.current.Symbols[node.Name.Value] = Symbol{
			Index: index, 
			Type: valueType, 
			IsParam: false,
			ShadowIndex: shadowIndex,
		}
		c.current.NextLocalID++
		c.current.ShadowStackSize++

		realIndex := index + c.current.ParamCount
		// local.set %d
		c.emit(fmt.Sprintf("local.set %d ;; %s (%s)", realIndex, node.Name.Value, valueType))
		
		// Push to shadow stack
		// store(shadow_stack_ptr, value)
		// shadow_stack_ptr += 4
		c.emit("global.get $shadow_stack_ptr")
		c.emit(fmt.Sprintf("local.get %d", realIndex))
		c.emit("i32.store")
		
		c.emit("global.get $shadow_stack_ptr")
		c.emit("i32.const 4")
		c.emit("i32.add")
		c.emit("global.set $shadow_stack_ptr")

	case *ast.ClassStatement:
		// Already handled in passes
		return nil

	case *ast.NewExpression:
		className := node.Class.Value
		classSym, ok := c.classes[className]
		if !ok {
			return fmt.Errorf("undefined class: %s", className)
		}

		// malloc(size)
		c.emit(fmt.Sprintf("i32.const %d", classSym.Size))
		c.emit(fmt.Sprintf("i32.const %d", classSym.TypeID))
		c.emit("call $malloc")

		// Check for constructor "init"
		if mangledName, ok := classSym.Methods["init"]; ok {
			// Store ptr in temp to use it multiple times
			tempIndex := c.current.NextLocalID
			c.current.NextLocalID++
			realTempIndex := tempIndex + c.current.ParamCount

			c.emit(fmt.Sprintf("local.set %d", realTempIndex))

			// Prepare 'this'
			c.emit(fmt.Sprintf("local.get %d", realTempIndex))

			// Args
			for _, arg := range node.Arguments {
				if err := c.Compile(arg); err != nil {
					return err
				}
			}

			c.emit(fmt.Sprintf("call $%s", mangledName))
			c.emit("drop") // Ignore init return value

			// Return instance
			c.emit(fmt.Sprintf("local.get %d", realTempIndex))
		} else if len(node.Arguments) > 0 {
			return fmt.Errorf("arguments provided for class %s but no 'init' method found", className)
		}

		c.stackType = TypeInt
		return nil

	case *ast.SuperExpression:
		// super.method() logic needs MemberExpression context, but if used alone?
		// Usually super() is for constructor.
		// super.foo() is for method access.
		// AST structure for super.foo() is MemberExpression(Object=SuperExpression, Property=foo)
		
		// If we are here, it means 'super' is used as a value, which is not really valid in this MVP except for member access.
		// But let's return 'this' pointer because super calls usually operate on 'this' instance.
		c.emit("local.get 0 ;; this (super)")
		c.stackType = TypeInt
		return nil

	case *ast.MemberExpression:
		// Check for Enum Access (e.g., Color.Red)
		if ident, ok := node.Object.(*ast.Identifier); ok {
			if members, isEnum := c.enums[ident.Value]; isEnum {
				val, ok := members[node.Property.Value]
				if !ok {
					return fmt.Errorf("enum %s has no member %s", ident.Value, node.Property.Value)
				}
				c.emit(fmt.Sprintf("i32.const %d", val))
				c.stackType = TypeInt
				return nil
			}
		}

		// Check for process.env
		if ident, ok := node.Object.(*ast.Identifier); ok && ident.Value == "process" && node.Property.Value == "env" {
			if c.target != "wasi" {
				return fmt.Errorf("process.env is only supported in WASI target")
			}
			c.emit("call $process_env")
			c.stackType = TypeMap
			return nil
		}

		// Special handling for super.method()

		if _, ok := node.Object.(*ast.SuperExpression); ok {
			if c.currentClass == "" {
				return fmt.Errorf("super used outside of class")
			}
			// We just let it fall through. compile(SuperExpression) puts 'this' on stack.
			// If it's a field access, the generic field lookup below will find it (inherited fields are copied).
			// If it's a method access not in a call, it will fail or behave weirdly, but we don't support function pointers yet.
		}

		if err := c.Compile(node.Object); err != nil {
			return err
		}
		
		propName := node.Property.Value
		
		// String.length
		if propName == "length" && c.stackType == TypeString {
			c.emit("call $strlen")
			c.stackType = TypeInt
			return nil
		}

		// Array.length
		if propName == "length" {
			// MVP: Assume if property is "length", it's array length
			// In strict mode, we should check c.stackType == TypeArray
			c.emit("call $array_length")
			c.stackType = TypeInt
			return nil
		}
		
		// Host Object Property Get
		if c.stackType == TypeHost {
			// call $host_get(handle, "prop")
			offset, ok := c.stringPool[propName]
			if !ok {
				offset = c.nextDataOffset
				c.stringPool[propName] = offset
				c.nextDataOffset += len(propName) + 1
			}
			c.emit(fmt.Sprintf("i32.const %d ;; \"%s\"", offset, propName))
			c.emit("call $host_get")
			c.stackType = TypeHost
			return nil
		}
		
		// MVP: Search for property in all known classes (since we lack full type system)
		// Better approach: Since we don't have type info on stack, we have to guess or search all classes.
		// If multiple classes have same field name but different types, we might have issues.
		// Ideally, we should track type of object on stack. But c.stackType is just DataType enum.
		// If c.stackType is TypeInt (pointer), we don't know which class it is.
		
		offset := -1
		var fieldType DataType = TypeInt
		
		found := false
		for _, cls := range c.classes {
			if off, ok := cls.Fields[propName]; ok {
				offset = off
				if ft, ok := cls.FieldTypes[propName]; ok {
					fieldType = ft
				}
				found = true
				break
			}
		}
		
		if !found {
			return fmt.Errorf("unknown property: %s", propName)
		}
		
		c.emit(fmt.Sprintf("i32.const %d", offset))
		c.emit("i32.add")
		c.emit("i32.load")
		c.stackType = fieldType
		return nil

	case *ast.AssignmentExpression:
		// Handle MemberExpression assignment: obj.prop = val
		if member, ok := node.Left.(*ast.MemberExpression); ok {
			if err := c.Compile(member.Object); err != nil {
				return err
			}
			targetType := c.stackType
			
			propName := member.Property.Value
			
			// Host Object Property Set
			if targetType == TypeHost {
				// call $host_set(handle, "prop", value)
				offset, ok := c.stringPool[propName]
				if !ok {
					offset = c.nextDataOffset
					c.stringPool[propName] = offset
					c.nextDataOffset += len(propName) + 1
				}
				c.emit(fmt.Sprintf("i32.const %d ;; \"%s\"", offset, propName))
				
				// Value
				if err := c.Compile(node.Value); err != nil {
					return err
				}
				valueType := c.stackType
				
				// Auto-convert value if needed
				if valueType == TypeString {
					c.emit("call $host_from_string")
				} else if valueType == TypeInt || valueType == TypeBool {
					c.emit("call $host_from_int")
				}
				// If it's TypeHost, it's already a handle
				
				c.emit("call $host_set")
				c.emit("i32.const 0")
				return nil
			}
			
			// Check if we can find property in classes
			offset := -1
			for _, cls := range c.classes {
				if off, ok := cls.Fields[propName]; ok {
					offset = off
					break
				}
			}
			
			if offset != -1 {
				c.emit(fmt.Sprintf("i32.const %d", offset))
				c.emit("i32.add")
				
				if err := c.Compile(node.Value); err != nil {
					return err
				}
				
				c.emit("i32.store")
				c.emit("i32.const 0") // Assignment returns 0 (void)
				return nil
			}
			
			return fmt.Errorf("unknown property in assignment: %s", propName)
		}
		
		// Handle IndexExpression assignment: arr[i] = val, map["k"] = val
		if indexExpr, ok := node.Left.(*ast.IndexExpression); ok {
			if err := c.Compile(indexExpr.Left); err != nil {
				return err
			}
			targetType := c.stackType
			
			if err := c.Compile(indexExpr.Index); err != nil {
				return err
			}
			indexType := c.stackType
			
			// Value to set
			if err := c.Compile(node.Value); err != nil {
				return err
			}
			
			if targetType == TypeArray {
				c.emit("call $array_set")
			} else if targetType == TypeMap || indexType == TypeString {
				c.emit("call $map_set")
			} else {
				// Fallback
				c.emit("call $array_set")
			}
			
			c.emit("i32.const 0")
			return nil
		}
		
		// Handle Identifier assignment: x = val
		if ident, ok := node.Left.(*ast.Identifier); ok {
			if err := c.Compile(node.Value); err != nil {
				return err
			}
			
			sym, ok := c.current.Symbols[ident.Value]
			if !ok {
				return fmt.Errorf("undefined variable: %s", ident.Value)
			}
			
			realIndex := sym.Index
			if !sym.IsParam {
				realIndex += c.current.ParamCount
			}
			
			c.emit(fmt.Sprintf("local.set %d", realIndex))
			c.emit(fmt.Sprintf("local.get %d", realIndex))
			
			// Update shadow stack: value = stack[shadowBase + shadowIndex*4]
			// We have shadowBase in local(shadowBaseIndex).
			shadowBaseIndex := c.current.ParamCount
			
			c.emit(fmt.Sprintf("local.get %d ;; shadow base", shadowBaseIndex))
			c.emit(fmt.Sprintf("i32.const %d", sym.ShadowIndex * 4))
			c.emit("i32.add")
			c.emit(fmt.Sprintf("local.get %d ;; value", realIndex))
			c.emit("i32.store")
			
			// Assignment returns the value (TS/JS behavior)
			// Stack already has the value from local.get above
			c.stackType = sym.Type
			return nil
		}
		
		return fmt.Errorf("invalid assignment target")

	case *ast.ThisExpression:
		// 'this' is always param 0
		c.emit("local.get 0 ;; this")
		c.stackType = TypeInt
		return nil

	case *ast.Identifier:
		sym, ok := c.current.Symbols[node.Value]
		if ok {
			realIndex := sym.Index
			if !sym.IsParam {
				realIndex += c.current.ParamCount
			}

			c.emit(fmt.Sprintf("local.get %d ;; %s (%s)", realIndex, node.Value, sym.Type))
			c.stackType = sym.Type
		} else {
			// If not found in locals, check if it's a known class (constructor) or global
			if _, ok := c.classes[node.Value]; ok {
				// It's a class name, but we are using it as a value?
				// Maybe static method call? Not supported yet.
				return fmt.Errorf("class usage as value not supported: %s", node.Value)
			}
			
			// Assume implicit global (Host Object)
			// call $host_get_global("name")
			offset, ok := c.stringPool[node.Value]
			if !ok {
				offset = c.nextDataOffset
				c.stringPool[node.Value] = offset
				c.nextDataOffset += len(node.Value) + 1
			}
			c.emit(fmt.Sprintf("i32.const %d ;; \"%s\"", offset, node.Value))
			c.emit("call $host_get_global")
			c.stackType = TypeHost
		}

	case *ast.PrefixExpression:
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		
		switch node.Operator {
		case "!":
			c.emit("i32.eqz")
			c.stackType = TypeBool
		case "-":
			if c.stackType != TypeInt {
				return fmt.Errorf("operator - not defined for type %s", c.stackType)
			}
			c.emit("i32.const -1")
			c.emit("i32.mul")
			c.stackType = TypeInt
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	case *ast.InfixExpression:
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		leftType := c.stackType

		if err := c.Compile(node.Right); err != nil {
			return err
		}
		rightType := c.stackType

		switch node.Operator {
		case "+":
			if leftType == TypeInt && rightType == TypeInt {
				c.emit("i32.add")
				c.stackType = TypeInt
			} else if leftType == TypeString && rightType == TypeString {
				c.emit("call $str_concat")
				c.stackType = TypeString
			} else if leftType == TypeString && rightType == TypeInt {
				// String + Int
				c.emit("call $itos")
				c.emit("call $str_concat")
				c.stackType = TypeString
			} else if leftType == TypeInt && rightType == TypeString {
				// Int + String
				// Swap via temp local
				tempIndex := c.current.NextLocalID
				c.current.NextLocalID++
				realTempIndex := tempIndex + c.current.ParamCount
				c.current.Symbols[fmt.Sprintf("$temp_swap_%d", tempIndex)] = Symbol{Index: tempIndex, Type: TypeString, IsParam: false, ShadowIndex: -1}
				
				c.emit(fmt.Sprintf("local.set %d", realTempIndex)) // Pop string
				c.emit("call $itos")                      // Convert int
				c.emit(fmt.Sprintf("local.get %d", realTempIndex)) // Push string back
				c.emit("call $str_concat")
				c.stackType = TypeString
			} else {
				return fmt.Errorf("operator + not defined for types %s and %s", leftType, rightType)
			}
		case "-":
			if leftType == TypeInt && rightType == TypeInt {
				c.emit("i32.sub")
				c.stackType = TypeInt
			} else {
				return fmt.Errorf("operator - not defined for types %s and %s", leftType, rightType)
			}
		case "*":
			if leftType == TypeInt && rightType == TypeInt {
				c.emit("i32.mul")
				c.stackType = TypeInt
			} else {
				return fmt.Errorf("operator * not defined for types %s and %s", leftType, rightType)
			}
		case "/":
			if leftType == TypeInt && rightType == TypeInt {
				c.emit("i32.div_s")
				c.stackType = TypeInt
			} else {
				return fmt.Errorf("operator / not defined for types %s and %s", leftType, rightType)
			}
		case "==":
			if leftType == rightType || (leftType == TypeHost && rightType == TypeInt) || (leftType == TypeInt && rightType == TypeHost) {
				c.emit("i32.eq")
				c.stackType = TypeBool
			} else {
				return fmt.Errorf("operator == not defined for types %s and %s", leftType, rightType)
			}
		case "!=":
			if leftType == rightType || (leftType == TypeHost && rightType == TypeInt) || (leftType == TypeInt && rightType == TypeHost) {
				c.emit("i32.ne")
				c.stackType = TypeBool
			} else {
				return fmt.Errorf("operator != not defined for types %s and %s", leftType, rightType)
			}
		case "<":
			if leftType == TypeInt && rightType == TypeInt {
				c.emit("i32.lt_s")
				c.stackType = TypeBool
			} else {
				return fmt.Errorf("operator < not defined for types %s and %s", leftType, rightType)
			}
		case ">":
			if leftType == TypeInt && rightType == TypeInt {
				c.emit("i32.gt_s")
				c.stackType = TypeBool
			} else {
				return fmt.Errorf("operator > not defined for types %s and %s", leftType, rightType)
			}
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	case *ast.CallExpression:
		if member, ok := node.Function.(*ast.MemberExpression); ok {
			// Check for super.method()
			if _, isSuper := member.Object.(*ast.SuperExpression); isSuper {
				if c.currentClass == "" {
					return fmt.Errorf("super call outside of class method")
				}
				
				currentSym := c.classes[c.currentClass]
				parentName := currentSym.Parent
				if parentName == "" {
					return fmt.Errorf("super call in class with no parent")
				}
				
				parentSym := c.classes[parentName]
				methodName := member.Property.Value
				
				mangledName, ok := parentSym.Methods[methodName]
				if !ok {
					return fmt.Errorf("method %s not found in parent class %s", methodName, parentName)
				}
				
				// Push 'this' (local 0) as first argument
				c.emit("local.get 0 ;; this (super)")
				
				// Compile other arguments
				for _, arg := range node.Arguments {
					if err := c.Compile(arg); err != nil {
						return err
					}
				}
				
				c.emit(fmt.Sprintf("call $%s", mangledName))
				c.stackType = TypeInt
				return nil
			}

			// Check for Array methods: push, pop (TODO)
			// Problem: We need to know if object is Array.
			// MVP: If method name is "push", treat as array push.
			if member.Property.Value == "push" {
				if err := c.Compile(member.Object); err != nil {
					return err
				}
				// Stack: [array_ptr]
				
				// Compile arguments (expect 1)
				if len(node.Arguments) != 1 {
					return fmt.Errorf("push expects 1 argument")
				}
				if err := c.Compile(node.Arguments[0]); err != nil {
					return err
				}
				
				c.emit("call $array_push")
				c.stackType = TypeVoid
				return nil
			}
			
			// Check for String methods: substring, charCodeAt
			if member.Property.Value == "substring" {
				if err := c.Compile(member.Object); err != nil {
					return err
				}
				// Stack: [str_ptr]
				
				if len(node.Arguments) != 2 {
					return fmt.Errorf("substring expects 2 arguments (start, end)")
				}
				
				if err := c.Compile(node.Arguments[0]); err != nil {
					return err
				}
				if err := c.Compile(node.Arguments[1]); err != nil {
					return err
				}
				
				c.emit("call $string_substring")
				c.stackType = TypeString
				return nil
			}
			
			if member.Property.Value == "charCodeAt" {
				if err := c.Compile(member.Object); err != nil {
					return err
				}
				// Stack: [str_ptr]
				
				if len(node.Arguments) != 1 {
					return fmt.Errorf("charCodeAt expects 1 argument (index)")
				}
				
				if err := c.Compile(node.Arguments[0]); err != nil {
					return err
				}
				
				c.emit("call $string_charCodeAt")
				c.stackType = TypeInt
				return nil
			}

			// Check for fs.writeFile / fs.writeFileSync (WASI only)
			if ident, ok := member.Object.(*ast.Identifier); ok && ident.Value == "fs" && (member.Property.Value == "writeFile" || member.Property.Value == "writeFileSync") {
				if c.target != "wasi" {
					return fmt.Errorf("fs.%s is only supported in WASI target", member.Property.Value)
				}
				
				if len(node.Arguments) < 2 || len(node.Arguments) > 3 {
					return fmt.Errorf("fs.%s expects 2 or 3 arguments (path, content, [options])", member.Property.Value)
				}
				
				// Compile path
				if err := c.Compile(node.Arguments[0]); err != nil {
					return err
				}
				
				// Compile content
				if err := c.Compile(node.Arguments[1]); err != nil {
					return err
				}

				// Ignore options for now (TODO: support options)
				
				c.emit("call $fs_writeFile")
				c.stackType = TypeVoid
				return nil
			}

			// Check for fs.readFile / fs.readFileSync (WASI only)
			if ident, ok := member.Object.(*ast.Identifier); ok && ident.Value == "fs" && (member.Property.Value == "readFile" || member.Property.Value == "readFileSync") {
				if c.target != "wasi" {
					return fmt.Errorf("fs.%s is only supported in WASI target", member.Property.Value)
				}
				
				if len(node.Arguments) < 1 || len(node.Arguments) > 2 {
					return fmt.Errorf("fs.%s expects 1 or 2 arguments (path, [options])", member.Property.Value)
				}
				
				// Compile path
				if err := c.Compile(node.Arguments[0]); err != nil {
					return err
				}
				
				// Ignore options for now
				
				c.emit("call $fs_readFile")
				c.stackType = TypeString
				return nil
			}

			// Check for fs.existsSync (WASI only)
			if ident, ok := member.Object.(*ast.Identifier); ok && ident.Value == "fs" && member.Property.Value == "existsSync" {
				if c.target != "wasi" {
					return fmt.Errorf("fs.existsSync is only supported in WASI target")
				}
				
				if len(node.Arguments) != 1 {
					return fmt.Errorf("fs.existsSync expects 1 argument (path)")
				}
				
				// Compile path
				if err := c.Compile(node.Arguments[0]); err != nil {
					return err
				}
				
				c.emit("call $fs_existsSync")
				c.stackType = TypeBool
				return nil
			}

			// Check for fs.unlinkSync (WASI only)
			if ident, ok := member.Object.(*ast.Identifier); ok && ident.Value == "fs" && member.Property.Value == "unlinkSync" {
				if c.target != "wasi" {
					return fmt.Errorf("fs.unlinkSync is only supported in WASI target")
				}
				
				if len(node.Arguments) != 1 {
					return fmt.Errorf("fs.unlinkSync expects 1 argument (path)")
				}
				
				if err := c.Compile(node.Arguments[0]); err != nil {
					return err
				}
				
				c.emit("call $fs_unlink")
				c.emit("drop") // Ignore result for now (void return in Node)
				c.stackType = TypeVoid
				return nil
			}

			// Check for fs.mkdirSync (WASI only)
			if ident, ok := member.Object.(*ast.Identifier); ok && ident.Value == "fs" && member.Property.Value == "mkdirSync" {
				if c.target != "wasi" {
					return fmt.Errorf("fs.mkdirSync is only supported in WASI target")
				}
				
				if len(node.Arguments) < 1 || len(node.Arguments) > 2 {
					return fmt.Errorf("fs.mkdirSync expects 1 or 2 arguments (path, [options])")
				}
				
				if err := c.Compile(node.Arguments[0]); err != nil {
					return err
				}
				
				c.emit("call $fs_mkdir")
				c.emit("drop")
				c.stackType = TypeVoid
				return nil
			}

			// Check for fs.rmdirSync (WASI only)
			if ident, ok := member.Object.(*ast.Identifier); ok && ident.Value == "fs" && member.Property.Value == "rmdirSync" {
				if c.target != "wasi" {
					return fmt.Errorf("fs.rmdirSync is only supported in WASI target")
				}
				
				if len(node.Arguments) != 1 {
					return fmt.Errorf("fs.rmdirSync expects 1 argument (path)")
				}
				
				if err := c.Compile(node.Arguments[0]); err != nil {
					return err
				}
				
				c.emit("call $fs_rmdir")
				c.emit("drop")
				c.stackType = TypeVoid
				return nil
			}

			// Check for std.args (WASI only)
			// Usage: std.args() -> Array<string>
			if ident, ok := member.Object.(*ast.Identifier); ok && ident.Value == "std" && member.Property.Value == "args" {
				if c.target != "wasi" {
					return fmt.Errorf("std.args is only supported in WASI target")
				}
				
				if len(node.Arguments) != 0 {
					return fmt.Errorf("std.args expects 0 arguments")
				}
				
				c.emit("call $std_args")
				c.stackType = TypeArray
				return nil
			}

			// Check for path module methods
			if ident, ok := member.Object.(*ast.Identifier); ok && ident.Value == "path" {
				switch member.Property.Value {
				case "basename":
					if len(node.Arguments) != 1 {
						return fmt.Errorf("path.basename expects 1 argument (path)")
					}
					if err := c.Compile(node.Arguments[0]); err != nil {
						return err
					}
					c.emit("call $path_basename")
					c.stackType = TypeString
					return nil

				case "dirname":
					if len(node.Arguments) != 1 {
						return fmt.Errorf("path.dirname expects 1 argument (path)")
					}
					if err := c.Compile(node.Arguments[0]); err != nil {
						return err
					}
					c.emit("call $path_dirname")
					c.stackType = TypeString
					return nil

				case "extname":
					if len(node.Arguments) != 1 {
						return fmt.Errorf("path.extname expects 1 argument (path)")
					}
					if err := c.Compile(node.Arguments[0]); err != nil {
						return err
					}
					c.emit("call $path_extname")
					c.stackType = TypeString
					return nil

				case "join":
					if len(node.Arguments) == 0 {
						return fmt.Errorf("path.join expects at least 1 argument")
					}
					
					// Compile first argument
					if err := c.Compile(node.Arguments[0]); err != nil {
						return err
					}
					
					// Iterate through rest arguments and call path_join2 iteratively
					for i := 1; i < len(node.Arguments); i++ {
						if err := c.Compile(node.Arguments[i]); err != nil {
							return err
						}
						c.emit("call $path_join2")
					}
					
					c.stackType = TypeString
					return nil
				}
			}

			// Check for process.exit
			if ident, ok := member.Object.(*ast.Identifier); ok && ident.Value == "process" && member.Property.Value == "exit" {
				if c.target != "wasi" {
					return fmt.Errorf("process.exit is only supported in WASI target")
				}
				if len(node.Arguments) != 1 {
					return fmt.Errorf("process.exit expects 1 argument (exit code)")
				}
				if err := c.Compile(node.Arguments[0]); err != nil {
					return err
				}
				c.emit("call $proc_exit")
				c.emit("unreachable") // Should not return
				c.stackType = TypeVoid
				return nil
			}


			// Check for process.env (WASI only)
			// Usage: process.env -> Map<string, string>
			// But here we are in CallExpression.
			// If user does `process.env()`, that's not Node.js like.
			// Node.js usage: `process.env` is an object.
			// But in our AST, if we access `process.env`, it is a MemberExpression.
			// If we do `process.env["KEY"]`, it is IndexExpression.
			// If we do `process.env.KEY`, it is MemberExpression.
			
			// This block is for CallExpression.
			// So `process.env()` calls. Not what we want.


			// Method call: obj.method(args)
			// 1. Compile obj to get 'this' pointer
			if err := c.Compile(member.Object); err != nil {
				return err
			}
			// Stack: [obj_ptr]
			targetType := c.stackType
			
			// Host Object Method Call
			if targetType == TypeHost {
				// We have object handle on stack.
				// Need to pack arguments into an array in Wasm memory.
				
				// 1. Allocate args array
				argCount := len(node.Arguments)
				
				// Create a temp local for args pointer
				tempName := fmt.Sprintf("$args_temp_%d", c.current.NextLocalID)
				tempIndex := c.current.NextLocalID
				c.current.Symbols[tempName] = Symbol{Index: tempIndex, Type: TypeInt, IsParam: false, ShadowIndex: -1} // No GC trace for temp buffer
				c.current.NextLocalID++
				
				if argCount > 0 {
					c.emit(fmt.Sprintf("i32.const %d", argCount*4))
					c.emit("i32.const 20") // Use ArrayData type for simplicity (ignored by GC mostly)
					c.emit("call $malloc")
					c.emit(fmt.Sprintf("local.set %d", tempIndex+c.current.ParamCount))
					
					// 2. Compile and store arguments
					for i, arg := range node.Arguments {
						// Get array ptr
						c.emit(fmt.Sprintf("local.get %d", tempIndex+c.current.ParamCount))
						c.emit(fmt.Sprintf("i32.const %d", i*4))
						c.emit("i32.add")
						
						// Compile Arg
						if err := c.Compile(arg); err != nil {
							return err
						}
						argType := c.stackType
						
						// Convert primitive to handle
						if argType == TypeString {
								c.emit("call $host_from_string")
							} else if argType == TypeInt || argType == TypeBool {
								c.emit("call $host_from_int")
							}
						
						// Store
						c.emit("i32.store")
					}
				} else {
					c.emit("i32.const 0")
					c.emit(fmt.Sprintf("local.set %d", tempIndex+c.current.ParamCount))
				}
				
				// Prepare call: host_call(handle, method_name, args_ptr, args_len)
				// Stack has [obj_handle] already (from compiling member.Object)
				// Wait, compiling member.Object put it on stack. But we interrupted flow to setup args.
				// We need to save obj_handle first!
				
				// Rework:
				// 1. Compile Object -> local.set $obj
				// 2. Allocate Args -> local.set $args
				// 3. Fill Args
				// 4. call host_call($obj, "method", $args, len)
				
				// Since we already compiled Object and it is on stack...
				// Let's store it in another temp
				objTempName := fmt.Sprintf("$obj_temp_%d", c.current.NextLocalID)
				objTempIndex := c.current.NextLocalID
				c.current.Symbols[objTempName] = Symbol{Index: objTempIndex, Type: TypeHost, IsParam: false, ShadowIndex: -1}
				c.current.NextLocalID++
				
				c.emit(fmt.Sprintf("local.set %d", objTempIndex+c.current.ParamCount))
				
				// Now allocate args (same as above)
				if argCount > 0 {
					c.emit(fmt.Sprintf("i32.const %d", argCount*4))
					c.emit("i32.const 20")
					c.emit("call $malloc")
					c.emit(fmt.Sprintf("local.set %d", tempIndex+c.current.ParamCount))
					
					for i, arg := range node.Arguments {
						c.emit(fmt.Sprintf("local.get %d", tempIndex+c.current.ParamCount))
						c.emit(fmt.Sprintf("i32.const %d", i*4))
						c.emit("i32.add")
						if err := c.Compile(arg); err != nil { return err }
							argType := c.stackType
							if argType == TypeString {
								c.emit("call $host_from_string")
							} else if argType == TypeInt || argType == TypeBool {
								c.emit("call $host_from_int")
							}
						c.emit("i32.store")
					}
				} else {
					c.emit("i32.const 0")
					c.emit(fmt.Sprintf("local.set %d", tempIndex+c.current.ParamCount))
				}
				
				// Call
				c.emit(fmt.Sprintf("local.get %d", objTempIndex+c.current.ParamCount))
				
				methodName := member.Property.Value
				offset, ok := c.stringPool[methodName]
				if !ok {
					offset = c.nextDataOffset
					c.stringPool[methodName] = offset
					c.nextDataOffset += len(methodName) + 1
				}
				c.emit(fmt.Sprintf("i32.const %d ;; \"%s\"", offset, methodName))
				
				c.emit(fmt.Sprintf("local.get %d", tempIndex+c.current.ParamCount))
				c.emit(fmt.Sprintf("i32.const %d", argCount))
				c.emit("call $host_call")
				c.stackType = TypeHost
				return nil
			}

			// Look up method name in ALL classes.
			methodName := member.Property.Value
			var mangledName string
			found := false
			for _, cls := range c.classes {
				if name, ok := cls.Methods[methodName]; ok {
					mangledName = name
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("unknown method: %s", methodName)
			}
			
			// Compile other arguments
			for _, arg := range node.Arguments {
				if err := c.Compile(arg); err != nil {
					return err
				}
			}
			
			c.emit(fmt.Sprintf("call $%s", mangledName))
			c.stackType = TypeInt
			return nil
		}

		if ident, ok := node.Function.(*ast.Identifier); ok {
			// Determine call type
			isLocalSymbol := false
			if _, ok := c.current.Symbols[ident.Value]; ok {
				isLocalSymbol = true
			}
			
			isImported := false
			if _, ok := c.importedFuncs[ident.Value]; ok {
				isImported = true
			}
			
			isDefined := false
			if _, ok := c.definedFuncs[ident.Value]; ok {
				isDefined = true
			}
			
			// 1. Local Symbol (e.g. host handle in var)
			if isLocalSymbol {
				if err := c.Compile(ident); err != nil { return err }
				if c.stackType == TypeHost {
					// Call host function handle
					handleTemp := c.current.NextLocalID
					c.current.NextLocalID++
					realHandleTemp := handleTemp + c.current.ParamCount
					c.emit(fmt.Sprintf("local.set %d", realHandleTemp))
					
					// Prepare args
					argCount := len(node.Arguments)
					argsTemp := c.current.NextLocalID
					c.current.NextLocalID++
					realArgsTemp := argsTemp + c.current.ParamCount
					
					if argCount > 0 {
						c.emit(fmt.Sprintf("i32.const %d", argCount*4))
						c.emit("i32.const 20")
						c.emit("call $malloc")
						c.emit(fmt.Sprintf("local.set %d", realArgsTemp))
						
						for i, arg := range node.Arguments {
							c.emit(fmt.Sprintf("local.get %d", realArgsTemp))
							c.emit(fmt.Sprintf("i32.const %d", i*4))
							c.emit("i32.add")
							if err := c.Compile(arg); err != nil { return err }
							argType := c.stackType
							if argType == TypeString {
								c.emit("call $host_from_string")
							} else if argType == TypeInt || argType == TypeBool {
								c.emit("call $host_from_int")
							}
							c.emit("i32.store")
						}
					} else {
						c.emit("i32.const 0")
						c.emit(fmt.Sprintf("local.set %d", realArgsTemp))
					}
					
					c.emit(fmt.Sprintf("local.get %d", realHandleTemp))
					c.emit("i32.const 0")
					c.emit(fmt.Sprintf("local.get %d", realArgsTemp))
					c.emit(fmt.Sprintf("i32.const %d", argCount))
					c.emit("call $host_call")
					c.stackType = TypeHost
					return nil
				}
				// Else: Local var but not TypeHost? Function pointer not supported.
				return fmt.Errorf("calling local variable %s of type %s not supported", ident.Value, c.stackType)
			}
			
			// 2. Imported Function
			if isImported {
				// Compile arguments normally (push to stack)
				for _, arg := range node.Arguments {
					if err := c.Compile(arg); err != nil { return err }
				}
				
				c.emit(fmt.Sprintf("call $%s", ident.Value))
				
				if c.importedFuncs[ident.Value].ReturnType != "void" {
					c.stackType = TypeInt
				} else {
					c.stackType = TypeVoid
				}
				return nil
			}
			
			// 3. Defined Internal Function (or stdlib)
			if isDefined {
				sig := c.definedFuncs[ident.Value]
				if len(node.Arguments) != len(sig.ParamTypes) {
					return fmt.Errorf("function %s expects %d arguments, got %d", ident.Value, len(sig.ParamTypes), len(node.Arguments))
				}

				for _, arg := range node.Arguments {
					if err := c.Compile(arg); err != nil { return err }
				}
				c.emit(fmt.Sprintf("call $%s", ident.Value))
				c.stackType = TypeInt // MVP assumption
				return nil
			}
			
			// 4. Implicit Global Host Call
			// If not local, not imported, not defined -> Host Call
			
			if c.target == "wasi" {
				if ident.Value == "print" {
					// Handle print via WASI
					arg := node.Arguments[0]
					if err := c.Compile(arg); err != nil { return err }
					
					// Assuming string argument for now
					// Prepare iovec for fd_write
					// [ptr, len]
					// We need to store iovec in memory. Let's use stack space (shadow stack base?) or allocate.
					// For MVP, allocate 8 bytes for iovec
					
					// Arg is on stack (pointer to string)
					// Get length
					
					// Hacky WASI print implementation for MVP
					// We need (local $str i32) (local $len i32) (local $iov i32) (local $written i32)
					// But we are in expression compilation, locals must be declared at top.
					// We can't easily add locals here.
					// Alternative: Call a helper function $wasi_print
					c.emit("call $wasi_print")
					c.stackType = TypeVoid
					return nil
				}
				if ident.Value == "int_to_string" {
					if len(node.Arguments) != 1 {
						return fmt.Errorf("int_to_string expects 1 argument")
					}
					if err := c.Compile(node.Arguments[0]); err != nil { return err }
					c.emit("call $itos")
					c.stackType = TypeString
					return nil
				}
				return fmt.Errorf("unknown function or global in WASI mode: %s", ident.Value)
			}

			// Get Global Handle
			offset, ok := c.stringPool[ident.Value]
			if !ok {
				offset = c.nextDataOffset
				c.stringPool[ident.Value] = offset
				c.nextDataOffset += len(ident.Value) + 1
			}
			c.emit(fmt.Sprintf("i32.const %d ;; \"%s\"", offset, ident.Value))
			c.emit("call $host_get_global")
			
			handleTemp := c.current.NextLocalID
			c.current.NextLocalID++
			realHandleTemp := handleTemp + c.current.ParamCount
			c.emit(fmt.Sprintf("local.set %d", realHandleTemp))
			
			argCount := len(node.Arguments)
			argsTemp := c.current.NextLocalID
			c.current.NextLocalID++
			realArgsTemp := argsTemp + c.current.ParamCount
			
			if argCount > 0 {
				c.emit(fmt.Sprintf("i32.const %d", argCount*4))
				c.emit("i32.const 20")
				c.emit("call $malloc")
				c.emit(fmt.Sprintf("local.set %d", realArgsTemp))
				
				for i, arg := range node.Arguments {
					c.emit(fmt.Sprintf("local.get %d", realArgsTemp))
					c.emit(fmt.Sprintf("i32.const %d", i*4))
					c.emit("i32.add")
					if err := c.Compile(arg); err != nil { return err }
					argType := c.stackType
					if argType == TypeString {
						c.emit("call $host_from_string")
					} else if argType == TypeInt || argType == TypeBool {
						c.emit("call $host_from_int")
					}
					c.emit("i32.store")
				}
			} else {
				c.emit("i32.const 0")
				c.emit(fmt.Sprintf("local.set %d", realArgsTemp))
			}
			
			c.emit(fmt.Sprintf("local.get %d", realHandleTemp))
			c.emit("i32.const 0")
			c.emit(fmt.Sprintf("local.get %d", realArgsTemp))
			c.emit(fmt.Sprintf("i32.const %d", argCount))
			c.emit("call $host_call")
			c.stackType = TypeHost
			return nil
		} else {
			// Compile arguments for generic call (if we fall here)
			for _, arg := range node.Arguments {
				if err := c.Compile(arg); err != nil { return err }
			}
			return fmt.Errorf("complex function calls not supported yet")
		}

	case *ast.IntegerLiteral:
		c.emit(fmt.Sprintf("i32.const %d", node.Value))
		c.stackType = TypeInt

	case *ast.StringLiteral:
		offset, ok := c.stringPool[node.Value]
		if !ok {
			offset = c.nextDataOffset
			c.stringPool[node.Value] = offset
			c.nextDataOffset += len(node.Value) + 1
		}
		c.emit(fmt.Sprintf("i32.const %d ;; pointer to \"%s\"", offset, node.Value))
		c.stackType = TypeString

	case *ast.ArrayLiteral:
		length := len(node.Elements)
		// Use dynamic array: $array_new(capacity)
		// For literal, use length as initial capacity
		
		c.emit(fmt.Sprintf("i32.const %d", length))
		c.emit("call $array_new")
		
		// Use a temporary local to store the array pointer so we can push elements
		tempName := fmt.Sprintf("$arr_temp_%d", c.current.NextLocalID)
		tempIndex := c.current.NextLocalID
		// Note: TypeArray is used for tracking, but shadow stack treats everything as pointer (int)
		c.current.Symbols[tempName] = Symbol{Index: tempIndex, Type: TypeArray, IsParam: false, ShadowIndex: c.current.ShadowStackSize}
		c.current.NextLocalID++
		c.current.ShadowStackSize++

		c.emit(fmt.Sprintf("local.set %d", tempIndex+c.current.ParamCount))
		
		// Push to shadow stack (it's a root!)
		c.emit("global.get $shadow_stack_ptr")
		c.emit(fmt.Sprintf("local.get %d", tempIndex+c.current.ParamCount))
		c.emit("i32.store")
		c.emit("global.get $shadow_stack_ptr")
		c.emit("i32.const 4")
		c.emit("i32.add")
		c.emit("global.set $shadow_stack_ptr")

		// Push elements
		for _, el := range node.Elements {
			// Prepare array ptr
			c.emit(fmt.Sprintf("local.get %d", tempIndex+c.current.ParamCount))
			
			// Compile value
			if err := c.Compile(el); err != nil {
				return err
			}
			
			// Call $array_push
			c.emit("call $array_push")
		}

		// Return array pointer
		c.emit(fmt.Sprintf("local.get %d", tempIndex+c.current.ParamCount))
		c.stackType = TypeArray

	case *ast.MapLiteral:
		c.emit("call $map_new")
		
		// Use a temporary local to store the map pointer
		tempName := fmt.Sprintf("$map_temp_%d", c.current.NextLocalID)
		tempIndex := c.current.NextLocalID
		c.current.Symbols[tempName] = Symbol{Index: tempIndex, Type: TypeMap, IsParam: false, ShadowIndex: c.current.ShadowStackSize}
		c.current.NextLocalID++
		c.current.ShadowStackSize++

		c.emit(fmt.Sprintf("local.set %d", tempIndex+c.current.ParamCount))
		
		// Push to shadow stack
		c.emit("global.get $shadow_stack_ptr")
		c.emit(fmt.Sprintf("local.get %d", tempIndex+c.current.ParamCount))
		c.emit("i32.store")
		c.emit("global.get $shadow_stack_ptr")
		c.emit("i32.const 4")
		c.emit("i32.add")
		c.emit("global.set $shadow_stack_ptr")
		
		// Set elements
		for key, val := range node.Pairs {
			// Prepare map ptr
			c.emit(fmt.Sprintf("local.get %d", tempIndex+c.current.ParamCount))
			
			// Compile Key
			if err := c.Compile(key); err != nil {
				return err
			}
			if c.stackType != TypeString {
				return fmt.Errorf("map keys must be strings")
			}
			
			// Compile Value
			if err := c.Compile(val); err != nil {
				return err
			}
			
			// Call $map_set
			c.emit("call $map_set")
		}
		
		// Return map pointer
		c.emit(fmt.Sprintf("local.get %d", tempIndex+c.current.ParamCount))
		c.stackType = TypeMap

	case *ast.IndexExpression:
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		
		targetType := c.stackType
		
		if err := c.Compile(node.Index); err != nil {
			return err
		}
		indexType := c.stackType

		if targetType == TypeArray {
			c.emit("call $array_get")
			c.stackType = TypeInt
		} else if targetType == TypeMap || indexType == TypeString {
			// If target is map OR index is string (duck typing for MVP)
			c.emit("call $map_get")
			c.stackType = TypeInt // Values are untyped int/ptr
		} else {
			// Fallback to array get if we don't know type (assume int index)
			c.emit("call $array_get")
			c.stackType = TypeInt
		}

	case *ast.Boolean:
		if node.Value {
			c.emit("i32.const 1")
		} else {
			c.emit("i32.const 0")
		}
		c.stackType = TypeBool

	case *ast.IfExpression:
		if err := c.Compile(node.Condition); err != nil {
			return err
		}

		c.emit("if (result i32)")

		if err := c.Compile(node.Consequence); err != nil {
			return err
		}
		// MVP: If blocks don't return values, push dummy 0
		c.emit("i32.const 0")

		c.emit("else")
		if node.Alternative != nil {
			if err := c.Compile(node.Alternative); err != nil {
				return err
			}
			c.emit("i32.const 0")
		} else {
			// Dummy return for else branch to satisfy type checker
			c.emit("i32.const 0")
		}

		c.emit("end")
		c.stackType = TypeInt

	case *ast.FunctionLiteral:
		// Should be handled by compileFunction called from Program
		return nil

	case *ast.BlockStatement:
		for _, stmt := range node.Statements {
			if err := c.Compile(stmt); err != nil {
				return err
			}
		}

	case *ast.ReturnStatement:
		if err := c.Compile(node.ReturnValue); err != nil {
			return err
		}
		c.emit("return")

	case *ast.WhileStatement:
		c.emit("block $break")
		c.emit("loop $continue")

		// Compile condition
		if err := c.Compile(node.Condition); err != nil {
			return err
		}

		// Check condition: if false (0), break
		c.emit("i32.eqz")
		c.emit("br_if $break")

		// Compile body
		if err := c.Compile(node.Body); err != nil {
			return err
		}

		// Jump back to start of loop
		c.emit("br $continue")

		c.emit("end") // end of loop
		c.emit("end") // end of block

	case *ast.ForStatement:
		// Init
		if node.Init != nil {
			if err := c.Compile(node.Init); err != nil {
				return err
			}
		}

		c.emit("block $break")
		c.emit("loop $top")

		// Condition
		if node.Condition != nil {
			if err := c.Compile(node.Condition); err != nil {
				return err
			}
			c.emit("i32.eqz")
			c.emit("br_if $break")
		}

		// Body
		if err := c.Compile(node.Body); err != nil {
			return err
		}

		// Update
		if node.Update != nil {
			if err := c.Compile(node.Update); err != nil {
				return err
			}
		}

		c.emit("br $top")
		c.emit("end")
		c.emit("end")

	}
	return nil
}

func (c *Compiler) compileFunction(fn *ast.FunctionLiteral) error {
	scope := NewFunctionScope(fn.Name)
	c.current = scope
	c.functions = append(c.functions, scope)

	// Save previous shadow stack pointer
	// We need a local for this
	// 1. Register parameters
	for i, param := range fn.Parameters {
		t := c.resolveType(param.Type)
		scope.Symbols[param.Name.Value] = Symbol{
			Index: i, 
			Type: t, 
			IsParam: true,
			ShadowIndex: scope.ShadowStackSize,
		}
		scope.ParamTypes = append(scope.ParamTypes, t)
		scope.ParamCount++
		scope.ShadowStackSize++
	}

	// 2. Save previous shadow stack pointer
	shadowPtrLocal := scope.NextLocalID
	scope.NextLocalID++
	realShadowPtrLocal := shadowPtrLocal + scope.ParamCount
	
	c.emit("global.get $shadow_stack_ptr")
	c.emit(fmt.Sprintf("local.set %d ;; save previous shadow_stack_ptr", realShadowPtrLocal))

	// 3. Push params to shadow stack
	for i := 0; i < scope.ParamCount; i++ {
		c.emit("global.get $shadow_stack_ptr")
		c.emit(fmt.Sprintf("local.get %d", i))
		c.emit("i32.store")
		
		c.emit("global.get $shadow_stack_ptr")
		c.emit("i32.const 4")
		c.emit("i32.add")
		c.emit("global.set $shadow_stack_ptr")
	}

	if err := c.Compile(fn.Body); err != nil {
		return err
	}

	// Restore shadow stack pointer
	c.emit(fmt.Sprintf("local.get %d", realShadowPtrLocal))
	c.emit("global.set $shadow_stack_ptr")

	// Implicit return 0 if no return statement (for void functions or just safety)
	c.emit("i32.const 0")
	return nil
}

func (c *Compiler) emit(instruction string) {
	if c.current != nil {
		c.current.Instructions = append(c.current.Instructions, instruction)
	}
}

// GenerateWAT returns the final WAT string
func (c *Compiler) GenerateWAT() string {
	var out bytes.Buffer
	out.WriteString("(module\n")

	if c.target == "wasi" {
		out.WriteString(`  (import "wasi_snapshot_preview1" "fd_write" (func $fd_write (param i32 i32 i32 i32) (result i32)))`)
		out.WriteString("\n")
		out.WriteString(`  (import "wasi_snapshot_preview1" "path_open" (func $path_open (param i32 i32 i32 i32 i32 i64 i64 i32 i32) (result i32)))`)
		out.WriteString("\n")
		out.WriteString(`  (import "wasi_snapshot_preview1" "fd_close" (func $fd_close (param i32) (result i32)))`)
		out.WriteString("\n")
		out.WriteString(`  (import "wasi_snapshot_preview1" "fd_read" (func $fd_read (param i32 i32 i32 i32) (result i32)))`)
		out.WriteString("\n")
		out.WriteString(`  (import "wasi_snapshot_preview1" "fd_filestat_get" (func $fd_filestat_get (param i32 i32) (result i32)))`)
		out.WriteString("\n")
		out.WriteString(`  (import "wasi_snapshot_preview1" "args_sizes_get" (func $args_sizes_get (param i32 i32) (result i32)))`)
		out.WriteString("\n")
		out.WriteString(`  (import "wasi_snapshot_preview1" "args_get" (func $args_get (param i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "environ_sizes_get" (func $environ_sizes_get (param i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "environ_get" (func $environ_get (param i32 i32) (result i32)))
  (import "env" "thread_spawn" (func $thread_spawn (param i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "path_unlink_file" (func $path_unlink_file (param i32 i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "path_create_directory" (func $path_create_directory (param i32 i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "path_remove_directory" (func $path_remove_directory (param i32 i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "path_filestat_get" (func $path_filestat_get (param i32 i32 i32 i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "proc_exit" (func $proc_exit (param i32)))`)
		out.WriteString("\n")

		// User defined imports for WASI
		for name, imported := range c.importedFuncs {
			params := ""
			for range imported.Parameters {
				params += " (param i32)"
			}
			result := ""
			if imported.ReturnType != "void" {
				result = " (result i32)"
			}
			out.WriteString(fmt.Sprintf("  (import \"env\" \"%s\" (func $%s%s%s))\n", name, name, params, result))
		}

	} else {
		// Browser imports
		for name, imported := range c.importedFuncs {
			params := ""
			for range imported.Parameters {
				params += " (param i32)" // All types are i32 for MVP
			}
			result := ""
			if imported.ReturnType != "void" {
				result = " (result i32)"
			}
			out.WriteString(fmt.Sprintf("  (import \"env\" \"%s\" (func $%s%s%s))\n", name, name, params, result))
		}
		
		// Host interop for browser
		out.WriteString("  (import \"env\" \"host_get_global\" (func $host_get_global (param i32) (result i32)))\n")
		out.WriteString("  (import \"env\" \"host_get\" (func $host_get (param i32) (param i32) (result i32)))\n")
		out.WriteString("  (import \"env\" \"host_set\" (func $host_set (param i32) (param i32) (param i32)))\n")
		out.WriteString("  (import \"env\" \"host_call\" (func $host_call (param i32) (param i32) (param i32) (param i32) (result i32)))\n")
		out.WriteString("  (import \"env\" \"host_from_int\" (func $host_from_int (param i32) (result i32)))\n")
		out.WriteString("  (import \"env\" \"host_from_string\" (func $host_from_string (param i32) (result i32)))\n")
		out.WriteString("  (import \"env\" \"host_to_int\" (func $host_to_int (param i32) (result i32)))\n")
	}

	out.WriteString("  (import \"env\" \"memory\" (memory 1 1000 shared))\n")
	out.WriteString("  (export \"memory\" (memory 0))\n")
	if c.target == "wasi" {
		// Reactor model: export _initialize, do NOT export _start
		out.WriteString("  (export \"_initialize\" (func $noop))\n")
        out.WriteString("  (export \"_set_stack_pointer\" (func $set_stack_pointer))\n")
	} else {
		out.WriteString("  (export \"gc\" (func $gc_collect))\n")
	}

	// Emit data segments for strings
	for str, offset := range c.stringPool {
		// Basic escaping for WAT
		escapedStr := fmt.Sprintf("%q", str)
		escapedStr = escapedStr[1 : len(escapedStr)-1] // Remove Go quotes
		out.WriteString(fmt.Sprintf("  (data (i32.const %d) \"%s\\00\")\n", offset, escapedStr))
	}

	// Emit Standard Library
	out.WriteString(stdLibWAT)
	out.WriteString("\n(func $noop)\n")
	
	if c.target == "wasi" {
		out.WriteString(`
(func $wasi_print (param $str i32)
  (local $len i32)
  (local $iov i32)
  (local $written i32)
  
  local.get $str
  call $strlen
  local.set $len
  
  ;; Allocate 8 bytes for iovec [ptr, len]
  i32.const 8
  i32.const 0
  call $malloc
  local.set $iov
  
  ;; Store ptr
  local.get $iov
  local.get $str
  i32.store
  
  ;; Store len
  local.get $iov
  i32.const 4
  i32.add
  local.get $len
  i32.store
  
  ;; call fd_write(1, iov, 1, written_ptr)
  i32.const 1 ;; stdout
  local.get $iov
  i32.const 1 ;; iovs_len
  local.get $iov ;; reuse iov ptr for written_ptr (dirty hack but safe if we don't read it)
  call $fd_write
  drop
  
  ;; Print newline
  i32.const 8
  i32.const 0
  call $malloc
  local.set $iov
  
  ;; Store ptr to newline (we need a newline string constant, but let's just make one on heap)
  i32.const 2
  i32.const 0
  call $malloc
  local.set $str
  local.get $str
  i32.const 10 ;; \n
  i32.store8
  local.get $str
  i32.const 1
  i32.add
  i32.const 0
  i32.store8
  
  local.get $iov
  local.get $str
  i32.store
  
  local.get $iov
  i32.const 4
  i32.add
  i32.const 1
  i32.store
  
  i32.const 1
  local.get $iov
  i32.const 1
  local.get $iov
  call $fd_write
  drop
)

(func $fs_writeFile (param $path i32) (param $content i32)
  (local $path_len i32)
  (local $fd_ptr i32)
  (local $fd i32)
  (local $content_len i32)
  (local $iovs i32)
  (local $nwritten i32)

  ;; Calculate path len
  local.get $path
  call $strlen
  local.set $path_len

  ;; Allocate fd_ptr (4 bytes)
  i32.const 4
  i32.const 0
  call $malloc
  local.set $fd_ptr

  ;; Call path_open
  ;; dirfd=3 (preopened .)
  i32.const 3 
  ;; dirflags=0
  i32.const 0
  ;; path
  local.get $path
  ;; path_len
  local.get $path_len
  ;; oflags=9 (CREAT|TRUNC)
  i32.const 9
  ;; rights_base=64 (WRITE) - Need i64
  i64.const 64
  ;; rights_inheriting=0 - Need i64
  i64.const 0
  ;; fd_flags=0
  i32.const 0
  ;; fd_ptr
  local.get $fd_ptr
  call $path_open
  drop ;; result errno

  ;; Get fd
  local.get $fd_ptr
  i32.load
  local.set $fd

  ;; Calculate content len
  local.get $content
  call $strlen
  local.set $content_len

  ;; Allocate iovs (8 bytes)
  i32.const 8
  i32.const 0
  call $malloc
  local.set $iovs

  ;; Fill iovs
  local.get $iovs
  local.get $content
  i32.store
  local.get $iovs
  i32.const 4
  i32.add
  local.get $content_len
  i32.store

  ;; Allocate nwritten (4 bytes)
  i32.const 4
  i32.const 0
  call $malloc
  local.set $nwritten

  ;; Call fd_write
  local.get $fd
  local.get $iovs
  i32.const 1
  local.get $nwritten
  call $fd_write
  drop

  ;; Call fd_close
  local.get $fd
  call $fd_close
  drop
)

(func $fs_readFile (param $path i32) (result i32)
  (local $path_len i32)
  (local $fd_ptr i32)
  (local $fd i32)
  (local $stat_ptr i32)
  (local $size i32)
  (local $str i32)
  (local $iovs i32)
  (local $nread i32)

  ;; Calculate path len
  local.get $path
  call $strlen
  local.set $path_len

  ;; Allocate fd_ptr
  i32.const 4
  i32.const 0
  call $malloc
  local.set $fd_ptr

  ;; path_open
  i32.const 3 ;; dirfd=.
  i32.const 0 ;; dirflags
  local.get $path
  local.get $path_len
  i32.const 0 ;; oflags
  i64.const 2097154 ;; rights_base=READ(2)|FD_FILESTAT_GET(2097152)
  i64.const 0 ;; rights_inheriting
  i32.const 0 ;; fd_flags
  local.get $fd_ptr
  call $path_open
  drop ;; result

  ;; Get fd
  local.get $fd_ptr
  i32.load
  local.set $fd

  ;; Get file stats (size)
  i32.const 64 ;; size of filestat
  i32.const 0
  call $malloc
  local.set $stat_ptr

  local.get $fd
  local.get $stat_ptr
  call $fd_filestat_get
  drop

  ;; Read size (offset 32, u64)
  local.get $stat_ptr
  i32.const 32
  i32.add
  i64.load
  i32.wrap_i64 ;; Assume file size < 4GB
  local.set $size

  ;; Allocate string buffer (size + 1 for null terminator)
  local.get $size
  i32.const 1
  i32.add
  i32.const 0 ;; TypeID 0 (String)
  call $malloc
  local.set $str

  ;; Allocate iovs
  i32.const 8
  i32.const 0
  call $malloc
  local.set $iovs

  ;; Fill iovs
  local.get $iovs
  local.get $str
  i32.store
  local.get $iovs
  i32.const 4
  i32.add
  local.get $size
  i32.store

  ;; Allocate nread
  i32.const 4
  i32.const 0
  call $malloc
  local.set $nread

  ;; fd_read
  local.get $fd
  local.get $iovs
  i32.const 1
  local.get $nread
  call $fd_read
  drop

  ;; Null terminate
  local.get $str
  local.get $size
  i32.add
  i32.const 0
  i32.store8

  ;; Close
  local.get $fd
  call $fd_close
  drop

  local.get $str
)

(func $fs_unlink (param $path i32) (result i32)
  (local $path_len i32)
  local.get $path
  call $strlen
  local.set $path_len
  
  i32.const 3 ;; dirfd
  local.get $path
  local.get $path_len
  call $path_unlink_file
)

(func $fs_mkdir (param $path i32) (result i32)
  (local $path_len i32)
  local.get $path
  call $strlen
  local.set $path_len
  
  i32.const 3 ;; dirfd
  local.get $path
  local.get $path_len
  call $path_create_directory
)

(func $fs_rmdir (param $path i32) (result i32)
  (local $path_len i32)
  local.get $path
  call $strlen
  local.set $path_len
  
  i32.const 3 ;; dirfd
  local.get $path
  local.get $path_len
  call $path_remove_directory
)

(func $fs_existsSync (param $path i32) (result i32)
  (local $path_len i32)
  (local $stat_buf i32)
  (local $res i32)
  
  local.get $path
  call $strlen
  local.set $path_len
  
  ;; Allocate stat buffer (64 bytes)
  i32.const 64
  i32.const 0
  call $malloc
  local.set $stat_buf
  
  i32.const 3 ;; dirfd
  i32.const 0 ;; flags (follow symlinks)
  local.get $path
  local.get $path_len
  local.get $stat_buf
  call $path_filestat_get
  local.set $res
  
  ;; 0 is success
  local.get $res
  i32.eqz
)

(func $std_args (result i32)
  (local $argc i32)
  (local $argv_buf_size i32)
  (local $argv_ptr i32)
  (local $argv_buf_ptr i32)
  (local $arr i32)
  (local $i i32)
  (local $arg_ptr i32)
  (local $arg_len i32)
  (local $str i32)
  (local $argc_ptr i32)
  (local $argv_buf_size_ptr i32)
  (local $dest i32)
  (local $src i32)
  (local $cnt i32)

  ;; Allocate space for argc and argv_buf_size
  i32.const 4
  i32.const 0
  call $malloc
  local.set $argc_ptr
  
  i32.const 4
  i32.const 0
  call $malloc
  local.set $argv_buf_size_ptr

  ;; Call args_sizes_get
  local.get $argc_ptr
  local.get $argv_buf_size_ptr
  call $args_sizes_get
  drop

  ;; Load argc and buf size
  local.get $argc_ptr
  i32.load
  local.set $argc
  
  local.get $argv_buf_size_ptr
  i32.load
  local.set $argv_buf_size

  ;; Allocate argv (array of pointers)
  local.get $argc
  i32.const 4
  i32.mul
  i32.const 0
  call $malloc
  local.set $argv_ptr

  ;; Allocate argv_buf (char buffer)
  local.get $argv_buf_size
  i32.const 0
  call $malloc
  local.set $argv_buf_ptr

  ;; Call args_get
  local.get $argv_ptr
  local.get $argv_buf_ptr
  call $args_get
  drop

  ;; Create Array to return
  local.get $argc
  call $array_new
  local.set $arr

  ;; Loop through argv and push to array
  i32.const 0
  local.set $i
  
  (block $done
    (loop $loop
      local.get $i
      local.get $argc
      i32.ge_u
      br_if $done

      ;; Get pointer to current arg string
      local.get $argv_ptr
      local.get $i
      i32.const 4
      i32.mul
      i32.add
      i32.load
      local.set $arg_ptr

      ;; We need to copy this to a managed string because argv_buf is a single block
      ;; and we want distinct string objects (TypeID 0).
      ;; Calculate length: iterate until null terminator
      local.get $arg_ptr
      call $strlen
      local.set $arg_len

      ;; Allocate new string
      local.get $arg_len
      i32.const 1
      i32.add
      i32.const 0 ;; TypeID 0
      call $malloc
      local.set $str

      ;; Copy chars
      ;; Inline memcpy
      local.get $arg_len
      i32.const 1
      i32.add ;; copy null terminator too
      local.set $cnt
      
      local.get $arg_ptr
      local.set $src
      
      local.get $str
      local.set $dest
      
      (block $cp_done
        (loop $cp_loop
           local.get $cnt
           i32.eqz
           br_if $cp_done
           
           local.get $dest
           local.get $src
           i32.load8_u
           i32.store8
           
           local.get $dest
           i32.const 1
           i32.add
           local.set $dest
           
           local.get $src
           i32.const 1
           i32.add
           local.set $src
           
           local.get $cnt
           i32.const 1
           i32.sub
           local.set $cnt
           br $cp_loop
        )
      )

      ;; Push to array
      local.get $arr
      local.get $str
      call $array_push

      local.get $i
      i32.const 1
      i32.add
      local.set $i
      br $loop
    )
  )

  local.get $arr
)
`)
	}

	// Emit GC Trace Function
	c.emitGCTrace(&out)

	for _, fn := range c.functions {
		// Export all functions for now (needed for spawn)
		exportName := fmt.Sprintf("(export \"%s\")", fn.Name)
		
		if fn.Name == "main" && c.target == "wasi" {
			// For WASI, main is exported as _start via the export section above.
			// We also export it as "main" via exportName above.
		}

		paramsStr := ""
		for i := 0; i < fn.ParamCount; i++ {
			paramsStr += " (param i32)"
		}

		out.WriteString(fmt.Sprintf("  (func $%s %s%s (result i32)\n", fn.Name, exportName, paramsStr))

		for i := 0; i < fn.NextLocalID; i++ {
			out.WriteString("    (local i32)\n")
		}
		
		for _, ins := range fn.Instructions {
			out.WriteString("    " + ins + "\n")
		}

		out.WriteString("  )\n")
	}

	out.WriteString(")\n")
	return out.String()
}

func (c *Compiler) emitGCTrace(out *bytes.Buffer) {
	out.WriteString("(func $gc_trace (param $ptr i32) (param $type_id i32)\n")
	out.WriteString("  (local $i i32)\n")
	out.WriteString("  (local $cnt i32)\n")
	
	// Create block for each class + default
	// br_table [default, array, class1, class2...]
	// TypeID: 0=String(ignore), 1=Array, 20=ArrayData, 2+=Classes
	
	// TypeID 1: Array (has data pointer at offset 8)
	out.WriteString("  local.get $type_id\n")
	out.WriteString("  i32.const 1\n")
	out.WriteString("  i32.eq\n")
	out.WriteString("  if\n")
	out.WriteString("    local.get $ptr\n")
	out.WriteString("    i32.const 8\n")
	out.WriteString("    i32.add\n")
	out.WriteString("    i32.load\n")
	out.WriteString("    call $gc_mark\n")
	out.WriteString("    return\n")
	out.WriteString("  end\n")

	// TypeID 20: ArrayData (buffer of elements)
	out.WriteString("  local.get $type_id\n")
	out.WriteString("  i32.const 20\n")
	out.WriteString("  i32.eq\n")
	out.WriteString("  if\n")
	// Scan elements. Size is in header (ptr-12). But header size is bytes.
	// We need element count = size / 4.
	out.WriteString("    local.get $ptr\n")
	out.WriteString("    i32.const 12\n")
	out.WriteString("    i32.sub\n")
	out.WriteString("    i32.load\n")
	out.WriteString("    i32.const 4\n")
	out.WriteString("    i32.div_u\n")
	out.WriteString("    local.set $cnt\n")
	
	out.WriteString("    i32.const 0\n")
	out.WriteString("    local.set $i\n")
	
	out.WriteString("    (block $done_trace\n")
	out.WriteString("      (loop $trace\n")
	out.WriteString("        local.get $i\n")
	out.WriteString("        local.get $cnt\n")
	out.WriteString("        i32.ge_u\n")
	out.WriteString("        br_if $done_trace\n")
	
	out.WriteString("        local.get $ptr\n")
	out.WriteString("        local.get $i\n")
	out.WriteString("        i32.const 4\n")
	out.WriteString("        i32.mul\n")
	out.WriteString("        i32.add\n")
	out.WriteString("        i32.load\n")
	out.WriteString("        call $gc_mark\n")
	
	out.WriteString("        local.get $i\n")
	out.WriteString("        i32.const 1\n")
	out.WriteString("        i32.add\n")
	out.WriteString("        local.set $i\n")
	out.WriteString("        br $trace\n")
	out.WriteString("      )\n")
	out.WriteString("    )\n")
	out.WriteString("    return\n")
	out.WriteString("  end\n")

	// TypeID 2: Map (capacity, count, buckets)
	out.WriteString("  local.get $type_id\n")
	out.WriteString("  i32.const 2\n")
	out.WriteString("  i32.eq\n")
	out.WriteString("  if\n")
	out.WriteString("    local.get $ptr\n")
	out.WriteString("    i32.const 8\n")
	out.WriteString("    i32.add\n")
	out.WriteString("    i32.load\n")
	out.WriteString("    call $gc_mark\n")
	out.WriteString("    return\n")
	out.WriteString("  end\n")

	// TypeID 21: MapBuckets (array of pointers) -> Same logic as ArrayData basically
	// Size is fixed 64 (16 buckets * 4 bytes) currently, but better to read size from header
	out.WriteString("  local.get $type_id\n")
	out.WriteString("  i32.const 21\n")
	out.WriteString("  i32.eq\n")
	out.WriteString("  if\n")
	out.WriteString("    local.get $ptr\n")
	out.WriteString("    i32.const 12\n")
	out.WriteString("    i32.sub\n")
	out.WriteString("    i32.load\n")
	out.WriteString("    i32.const 4\n")
	out.WriteString("    i32.div_u\n")
	out.WriteString("    local.set $cnt\n")
	
	out.WriteString("    i32.const 0\n")
	out.WriteString("    local.set $i\n")
	
	out.WriteString("    (block $done_trace\n")
	out.WriteString("      (loop $trace\n")
	out.WriteString("        local.get $i\n")
	out.WriteString("        local.get $cnt\n")
	out.WriteString("        i32.ge_u\n")
	out.WriteString("        br_if $done_trace\n")
	
	out.WriteString("        local.get $ptr\n")
	out.WriteString("        local.get $i\n")
	out.WriteString("        i32.const 4\n")
	out.WriteString("        i32.mul\n")
	out.WriteString("        i32.add\n")
	out.WriteString("        i32.load\n")
	out.WriteString("        call $gc_mark\n")
	
	out.WriteString("        local.get $i\n")
	out.WriteString("        i32.const 1\n")
	out.WriteString("        i32.add\n")
	out.WriteString("        local.set $i\n")
	out.WriteString("        br $trace\n")
	out.WriteString("      )\n")
	out.WriteString("    )\n")
	out.WriteString("    return\n")
	out.WriteString("  end\n")

	// TypeID 22: MapEntry (key, value, next)
	out.WriteString("  local.get $type_id\n")
	out.WriteString("  i32.const 22\n")
	out.WriteString("  i32.eq\n")
	out.WriteString("  if\n")
	out.WriteString("    local.get $ptr\n")
	out.WriteString("    i32.load\n")
	out.WriteString("    call $gc_mark\n") // key
	
	out.WriteString("    local.get $ptr\n")
	out.WriteString("    i32.const 4\n")
	out.WriteString("    i32.add\n")
	out.WriteString("    i32.load\n")
	out.WriteString("    call $gc_mark\n") // value
	
	out.WriteString("    local.get $ptr\n")
	out.WriteString("    i32.const 8\n")
	out.WriteString("    i32.add\n")
	out.WriteString("    i32.load\n")
	out.WriteString("    call $gc_mark\n") // next
	out.WriteString("    return\n")
	out.WriteString("  end\n")

	for _, cls := range c.classes {
		out.WriteString(fmt.Sprintf("  ;; Class %s (TypeID %d)\n", cls.Name, cls.TypeID))
		out.WriteString("  local.get $type_id\n")
		out.WriteString(fmt.Sprintf("  i32.const %d\n", cls.TypeID))
		out.WriteString("  i32.eq\n")
		out.WriteString("  if\n")
		
		// Scan fields
		for name, offset := range cls.Fields {
			fieldType := cls.FieldTypes[name]
			// Only trace pointer types
			// Assuming Int/Bool are values. String is pointer but leaf (no trace inside).
			// Array and Class types need tracing.
			// How do we know if a field is a Class type?
			// DataType is string. If it's not int/bool/string/void, it's a class or array.
			
			if fieldType != TypeInt && fieldType != TypeBool && fieldType != TypeString && fieldType != TypeVoid {
				out.WriteString(fmt.Sprintf("    ;; Field %s (offset %d)\n", name, offset))
				out.WriteString("    local.get $ptr\n")
				out.WriteString(fmt.Sprintf("    i32.const %d\n", offset))
				out.WriteString("    i32.add\n")
				out.WriteString("    i32.load\n")
				out.WriteString("    call $gc_mark\n")
			}
		}
		
		out.WriteString("    return\n")
		out.WriteString("  end\n")
	}
	
	out.WriteString(")\n")
}

func (c *Compiler) defineEnum(node *ast.EnumStatement) error {
	enumName := node.Name.Value
	if _, exists := c.enums[enumName]; exists {
		return fmt.Errorf("enum %s already defined", enumName)
	}

	members := make(map[string]int)
	currentValue := 0

	for _, member := range node.Members {
		if member.Value != nil {
			// Evaluate constant value
			// For now, only support integer literals
			if intLit, ok := member.Value.(*ast.IntegerLiteral); ok {
				currentValue = int(intLit.Value)
			} else {
				return fmt.Errorf("enum member value must be integer literal")
			}
		}
		
		members[member.Name.Value] = currentValue
		currentValue++
	}

	c.enums[enumName] = members
	return nil
}

func (c *Compiler) defineClass(node *ast.ClassStatement) error {
	className := node.Name.Value
	classSymbol := ClassSymbol{
		Name:       className,
		Fields:     make(map[string]int),
		FieldTypes: make(map[string]DataType),
		Methods:    make(map[string]string),
		TypeID:     c.nextTypeID,
	}
	c.nextTypeID++

	offset := 0

	// Inherit from parent
	if node.Parent != nil {
		parentName := node.Parent.Value
		parentSym, ok := c.classes[parentName]
		if !ok {
			return fmt.Errorf("undefined parent class: %s", parentName)
		}
		classSymbol.Parent = parentName
		
		// Copy fields
		for k, v := range parentSym.Fields {
			classSymbol.Fields[k] = v
		}
		for k, v := range parentSym.FieldTypes {
			classSymbol.FieldTypes[k] = v
		}
		offset = parentSym.Size
		
		// Copy methods
		for k, v := range parentSym.Methods {
			classSymbol.Methods[k] = v
		}
	}

	// Add new fields
	for _, field := range node.Fields {
		classSymbol.Fields[field.Name.Value] = offset
		
		// Parse type
		dataType := c.resolveType(field.Type)
		classSymbol.FieldTypes[field.Name.Value] = dataType
		
		offset += 4 
	}
	classSymbol.Size = offset

	// Add/Override methods
	for _, method := range node.Methods {
		mangledName := fmt.Sprintf("%s_%s", className, method.Name)
		classSymbol.Methods[method.Name] = mangledName
	}

	c.classes[className] = classSymbol
	return nil
}

func (c *Compiler) compileClassMethods(node *ast.ClassStatement) error {
	className := node.Name.Value
	c.currentClass = className
	defer func() { c.currentClass = "" }()

	for _, method := range node.Methods {
		mangledName := fmt.Sprintf("%s_%s", className, method.Name)

		scope := NewFunctionScope(mangledName)
		c.current = scope
		c.functions = append(c.functions, scope)

		// Add 'this' parameter as first parameter
		scope.Symbols["this"] = Symbol{Index: 0, Type: TypeInt, IsParam: true}
		scope.ParamTypes = append(scope.ParamTypes, TypeInt)
		scope.ParamCount++

		for i, param := range method.Parameters {
			t := c.resolveType(param.Type)
			scope.Symbols[param.Name.Value] = Symbol{Index: i + 1, Type: t, IsParam: true}
			scope.ParamTypes = append(scope.ParamTypes, t)
			scope.ParamCount++
		}

		if err := c.Compile(method.Body); err != nil {
			return err
		}

		c.emit("i32.const 0")
	}
	return nil
}

func (c *Compiler) defineInterface(node *ast.InterfaceStatement) error {
	if _, ok := c.interfaces[node.Name.Value]; ok {
		return fmt.Errorf("interface %s already defined", node.Name.Value)
	}
	
	sym := InterfaceSymbol{
		Name:    node.Name.Value,
		Methods: make(map[string]*ast.MethodSignature),
	}
	
	for _, m := range node.Methods {
		sym.Methods[m.Name] = m
	}
	
	c.interfaces[node.Name.Value] = sym
	return nil
}

func (c *Compiler) checkInterfaceImplementation(node *ast.ClassStatement) error {
	className := node.Name.Value
	classSym, ok := c.classes[className]
	if !ok { return fmt.Errorf("internal error: class %s not found", className) }

	for _, impl := range node.Implements {
		ifaceName := impl.Value
		ifaceSym, ok := c.interfaces[ifaceName]
		if !ok {
			return fmt.Errorf("class %s implements undefined interface %s", className, ifaceName)
		}
		
		for methodName := range ifaceSym.Methods {
			if _, hasMethod := classSym.Methods[methodName]; !hasMethod {
				return fmt.Errorf("class %s does not implement method %s from interface %s", className, methodName, ifaceName)
			}
		}
	}
	return nil
}
