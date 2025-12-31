(module $main
  (type (;0;) (func (param i32 i32 i32 i32) (result i32)))
  (type (;1;) (func (param i32)))
  (type (;2;) (func (param i32 i32) (result i32)))
  (type (;3;) (func (result i32)))
  (type (;4;) (func (param i32) (result i32)))
  (type (;5;) (func (param i32 i32)))
  (type (;6;) (func (param i32 i32 i32)))
  (type (;7;) (func))
  (type (;8;) (func (param i32 i32 i32 i32)))
  (type (;9;) (func (result i64)))
  (type (;10;) (func (param i32 i32 i32) (result i32)))
  (type (;11;) (func (param i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32)))
  (type (;12;) (func (param i32 i64)))
  (type (;13;) (func (param i32 i32 i32 i32 i32 i32)))
  (type (;14;) (func (param i32 i32 i32 i32 i32)))
  (type (;15;) (func (param i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32)))
  (import "env" "malloc" (func $github.com/weisyn/v1/contracts/sdk/go/framework.malloc (type 4)))
  (import "env" "get_timestamp" (func $github.com/weisyn/v1/contracts/sdk/go/framework.getTimestamp (type 9)))
  (import "env" "get_caller" (func $github.com/weisyn/v1/contracts/sdk/go/framework.getCaller (type 4)))
  (import "env" "get_block_height" (func $github.com/weisyn/v1/contracts/sdk/go/framework.getBlockHeight (type 9)))
  (import "env" "get_contract_init_params" (func $github.com/weisyn/v1/contracts/sdk/go/framework.getContractInitParams (type 2)))
  (import "env" "set_return_data" (func $github.com/weisyn/v1/contracts/sdk/go/framework.setReturnData (type 2)))
  (import "wasi_snapshot_preview1" "fd_write" (func $runtime.fd_write (type 0)))
  (import "wasi_snapshot_preview1" "proc_exit" (func $runtime.proc_exit (type 1)))
  (import "wasi_snapshot_preview1" "random_get" (func $__imported_wasi_snapshot_preview1_random_get (type 2)))
  (func $chacha20_rng (type 1) (param i32)
    (local i32 i32)
    global.get $__stack_pointer
    i32.const -64
    i32.add
    local.tee 1
    global.set $__stack_pointer
    local.get 1
    i32.const 56
    i32.add
    i64.const 0
    i64.store
    local.get 1
    i32.const 24
    i32.add
    i32.const 66680
    i64.load align=1
    i64.store
    local.get 1
    i32.const 32
    i32.add
    i32.const 66688
    i64.load align=1
    i64.store
    local.get 1
    i32.const 40
    i32.add
    i32.const 66696
    i64.load align=1
    i64.store
    local.get 1
    i64.const 0
    i64.store offset=48
    local.get 1
    i32.const 65544
    i64.load
    i64.store offset=8
    local.get 1
    i32.const 65536
    i64.load
    i64.store
    local.get 1
    i32.const 66672
    i64.load align=1
    i64.store offset=16
    local.get 0
    local.get 1
    call $chacha20_update
    i32.const 66696
    local.get 0
    i32.const 24
    i32.add
    i64.load align=1
    i64.store align=1
    i32.const 66688
    local.get 0
    i32.const 16
    i32.add
    i64.load align=1
    i64.store align=1
    i32.const 66680
    local.get 0
    i32.const 8
    i32.add
    i64.load align=1
    i64.store align=1
    i32.const 66672
    local.get 0
    i64.load align=1
    i64.store align=1
    loop  ;; label = @1
      local.get 2
      i32.const 512
      i32.ne
      if  ;; label = @2
        local.get 0
        local.get 2
        i32.add
        local.get 1
        call $chacha20_update
        local.get 2
        i32.const -64
        i32.sub
        local.set 2
        br 1 (;@1;)
      end
    end
    local.get 1
    i32.const -64
    i32.sub
    global.set $__stack_pointer)
  (func $chacha20_update (type 5) (param i32 i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const -64
    i32.add
    local.tee 20
    local.get 1
    i32.const 64
    memory.copy
    local.get 1
    i32.load offset=44
    local.set 7
    local.get 1
    i32.load offset=60
    local.set 8
    local.get 1
    i32.load offset=12
    local.set 14
    local.get 1
    i32.load offset=28
    local.set 2
    local.get 1
    i32.load offset=40
    local.set 5
    local.get 1
    i32.load offset=56
    local.set 15
    local.get 1
    i32.load offset=8
    local.set 10
    local.get 1
    i32.load offset=24
    local.set 3
    local.get 1
    i32.load offset=36
    local.set 11
    local.get 1
    i32.load offset=52
    local.set 16
    local.get 1
    i32.load offset=4
    local.set 17
    local.get 1
    i32.load offset=20
    local.set 4
    local.get 1
    i32.load offset=32
    local.set 12
    local.get 1
    i32.load offset=48
    local.set 13
    local.get 1
    i32.load
    local.set 9
    local.get 1
    i32.load offset=16
    local.set 6
    loop  ;; label = @1
      local.get 18
      i32.const 19
      i32.gt_u
      i32.eqz
      if  ;; label = @2
        local.get 6
        local.get 12
        local.get 6
        local.get 9
        i32.add
        local.tee 6
        local.get 13
        i32.xor
        i32.const 16
        i32.rotl
        local.tee 12
        i32.add
        local.tee 13
        i32.xor
        i32.const 12
        i32.rotl
        local.tee 9
        local.get 6
        i32.add
        local.tee 19
        local.get 12
        i32.xor
        i32.const 8
        i32.rotl
        local.tee 21
        local.get 13
        i32.add
        local.tee 12
        local.get 9
        i32.xor
        i32.const 7
        i32.rotl
        local.tee 6
        local.get 2
        local.get 7
        local.get 2
        local.get 14
        i32.add
        local.tee 2
        local.get 8
        i32.xor
        i32.const 16
        i32.rotl
        local.tee 7
        i32.add
        local.tee 8
        i32.xor
        i32.const 12
        i32.rotl
        local.tee 13
        local.get 2
        i32.add
        local.tee 2
        i32.add
        local.tee 14
        local.get 3
        local.get 5
        local.get 3
        local.get 10
        i32.add
        local.tee 3
        local.get 15
        i32.xor
        i32.const 16
        i32.rotl
        local.tee 5
        i32.add
        local.tee 9
        i32.xor
        i32.const 12
        i32.rotl
        local.tee 22
        local.get 3
        i32.add
        local.tee 3
        local.get 5
        i32.xor
        i32.const 8
        i32.rotl
        local.tee 5
        i32.xor
        i32.const 16
        i32.rotl
        local.tee 15
        local.get 4
        local.get 4
        local.get 17
        i32.add
        local.tee 4
        local.get 16
        i32.xor
        i32.const 16
        i32.rotl
        local.tee 10
        local.get 11
        i32.add
        local.tee 11
        i32.xor
        i32.const 12
        i32.rotl
        local.tee 23
        local.get 4
        i32.add
        local.tee 4
        local.get 10
        i32.xor
        i32.const 8
        i32.rotl
        local.tee 10
        local.get 11
        i32.add
        local.tee 24
        i32.add
        local.tee 11
        local.get 6
        i32.xor
        i32.const 12
        i32.rotl
        local.tee 6
        local.get 14
        i32.add
        local.tee 14
        local.get 15
        i32.xor
        i32.const 8
        i32.rotl
        local.tee 15
        local.get 11
        i32.add
        local.tee 11
        local.get 6
        i32.xor
        i32.const 7
        i32.rotl
        local.set 6
        local.get 12
        local.get 10
        local.get 3
        local.get 8
        local.get 2
        local.get 7
        i32.xor
        i32.const 8
        i32.rotl
        local.tee 8
        i32.add
        local.tee 3
        local.get 13
        i32.xor
        i32.const 7
        i32.rotl
        local.tee 2
        i32.add
        local.tee 7
        i32.xor
        i32.const 16
        i32.rotl
        local.tee 16
        i32.add
        local.tee 17
        local.get 2
        i32.xor
        i32.const 12
        i32.rotl
        local.tee 2
        local.get 7
        i32.add
        local.tee 10
        local.get 16
        i32.xor
        i32.const 8
        i32.rotl
        local.tee 16
        local.get 17
        i32.add
        local.tee 12
        local.get 2
        i32.xor
        i32.const 7
        i32.rotl
        local.set 2
        local.get 3
        local.get 21
        local.get 4
        local.get 5
        local.get 9
        i32.add
        local.tee 4
        local.get 22
        i32.xor
        i32.const 7
        i32.rotl
        local.tee 3
        i32.add
        local.tee 7
        i32.xor
        i32.const 16
        i32.rotl
        local.tee 5
        i32.add
        local.tee 9
        local.get 3
        i32.xor
        i32.const 12
        i32.rotl
        local.tee 3
        local.get 7
        i32.add
        local.tee 17
        local.get 5
        i32.xor
        i32.const 8
        i32.rotl
        local.tee 13
        local.get 9
        i32.add
        local.tee 7
        local.get 3
        i32.xor
        i32.const 7
        i32.rotl
        local.set 3
        local.get 4
        local.get 8
        local.get 23
        local.get 24
        i32.xor
        i32.const 7
        i32.rotl
        local.tee 4
        local.get 19
        i32.add
        local.tee 8
        i32.xor
        i32.const 16
        i32.rotl
        local.tee 5
        i32.add
        local.tee 19
        local.get 4
        i32.xor
        i32.const 12
        i32.rotl
        local.tee 4
        local.get 8
        i32.add
        local.tee 9
        local.get 5
        i32.xor
        i32.const 8
        i32.rotl
        local.tee 8
        local.get 19
        i32.add
        local.tee 5
        local.get 4
        i32.xor
        i32.const 7
        i32.rotl
        local.set 4
        local.get 18
        i32.const 2
        i32.add
        local.set 18
        br 1 (;@1;)
      end
    end
    local.get 1
    local.get 13
    i32.store offset=48
    local.get 1
    local.get 9
    i32.store
    local.get 1
    local.get 6
    i32.store offset=16
    local.get 1
    local.get 12
    i32.store offset=32
    local.get 1
    local.get 4
    i32.store offset=20
    local.get 1
    local.get 16
    i32.store offset=52
    local.get 1
    local.get 17
    i32.store offset=4
    local.get 1
    local.get 11
    i32.store offset=36
    local.get 1
    local.get 3
    i32.store offset=24
    local.get 1
    local.get 15
    i32.store offset=56
    local.get 1
    local.get 10
    i32.store offset=8
    local.get 1
    local.get 5
    i32.store offset=40
    local.get 1
    local.get 2
    i32.store offset=28
    local.get 1
    local.get 8
    i32.store offset=60
    local.get 1
    local.get 14
    i32.store offset=12
    local.get 1
    local.get 7
    i32.store offset=44
    i32.const 0
    local.set 2
    loop  ;; label = @1
      local.get 2
      i32.const 64
      i32.eq
      i32.eqz
      if  ;; label = @2
        local.get 2
        local.get 20
        i32.add
        local.tee 3
        local.get 3
        i32.load
        local.get 1
        local.get 2
        i32.add
        i32.load
        i32.add
        i32.store
        local.get 2
        i32.const 4
        i32.add
        local.set 2
        br 1 (;@1;)
      end
    end
    local.get 0
    local.get 20
    i32.const 64
    memory.copy
    local.get 1
    local.get 1
    i32.load offset=48
    i32.const 1
    i32.add
    i32.store offset=48)
  (func $arc4random (type 3) (result i32)
    (local i32 i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const 16
    i32.sub
    local.tee 3
    global.set $__stack_pointer
    local.get 3
    i32.const 12
    i32.add
    local.set 4
    i32.const 4
    local.set 1
    i32.const 66664
    i32.load
    i32.eqz
    if  ;; label = @1
      i32.const 66672
      i32.const 32
      call $__imported_wasi_snapshot_preview1_random_get
      i32.const 65535
      i32.and
      local.tee 0
      if  ;; label = @2
        i32.const 67216
        local.get 0
        i32.store
      end
      i32.const 66664
      i64.const 2199023255553
      i64.store align=4
    end
    loop  ;; label = @1
      block  ;; label = @2
        local.get 1
        i32.eqz
        br_if 0 (;@2;)
        i32.const 66668
        i32.load
        local.tee 0
        i32.const 512
        i32.eq
        if  ;; label = @3
          loop  ;; label = @4
            local.get 1
            i32.const 512
            i32.ge_u
            if  ;; label = @5
              local.get 2
              local.get 4
              i32.add
              call $chacha20_rng
              local.get 1
              i32.const 512
              i32.sub
              local.set 1
              local.get 2
              i32.const 512
              i32.add
              local.set 2
              br 1 (;@4;)
            end
          end
          local.get 1
          i32.eqz
          br_if 1 (;@2;)
          i32.const 66704
          call $chacha20_rng
          i32.const 66668
          i32.const 0
          i32.store
          i32.const 0
          local.set 0
        end
        local.get 2
        local.get 4
        i32.add
        local.get 0
        i32.const 66704
        i32.add
        local.get 1
        i32.const 512
        local.get 0
        i32.sub
        local.tee 0
        local.get 0
        local.get 1
        i32.gt_u
        select
        local.tee 0
        memory.copy
        i32.const 66668
        i32.load
        i32.const 66704
        i32.add
        i32.const 0
        local.get 0
        memory.fill
        i32.const 66668
        i32.const 66668
        i32.load
        local.get 0
        i32.add
        i32.store
        local.get 0
        local.get 2
        i32.add
        local.set 2
        local.get 1
        local.get 0
        i32.sub
        local.set 1
        br 1 (;@1;)
      end
    end
    local.get 3
    i32.load offset=12
    local.get 3
    i32.const 16
    i32.add
    global.set $__stack_pointer)
  (func $_github.com/weisyn/v1/contracts/sdk/go/framework.Address_.ToString (type 11) (param i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32)
    (local i32 i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const 96
    i32.sub
    local.tee 21
    global.set $__stack_pointer
    local.get 21
    i64.const 1
    i64.store offset=84 align=4
    i32.const 67404
    i32.load
    local.set 24
    i32.const 67404
    local.get 21
    i32.const 80
    i32.add
    i32.store
    local.get 21
    local.get 24
    i32.store offset=80
    local.get 21
    i32.const 40
    i32.add
    i32.const 0
    i32.const 40
    memory.fill
    local.get 21
    i32.const 30768
    i32.store16 offset=38 align=1
    local.get 21
    i32.const 41
    i32.add
    local.set 23
    loop  ;; label = @1
      local.get 22
      i32.const 20
      i32.eq
      i32.eqz
      if  ;; label = @2
        local.get 21
        local.get 20
        i32.store8 offset=37
        local.get 21
        local.get 19
        i32.store8 offset=36
        local.get 21
        local.get 18
        i32.store8 offset=35
        local.get 21
        local.get 17
        i32.store8 offset=34
        local.get 21
        local.get 16
        i32.store8 offset=33
        local.get 21
        local.get 15
        i32.store8 offset=32
        local.get 21
        local.get 14
        i32.store8 offset=31
        local.get 21
        local.get 13
        i32.store8 offset=30
        local.get 21
        local.get 12
        i32.store8 offset=29
        local.get 21
        local.get 11
        i32.store8 offset=28
        local.get 21
        local.get 10
        i32.store8 offset=27
        local.get 21
        local.get 9
        i32.store8 offset=26
        local.get 21
        local.get 8
        i32.store8 offset=25
        local.get 21
        local.get 7
        i32.store8 offset=24
        local.get 21
        local.get 6
        i32.store8 offset=23
        local.get 21
        local.get 5
        i32.store8 offset=22
        local.get 21
        local.get 4
        i32.store8 offset=21
        local.get 21
        local.get 3
        i32.store8 offset=20
        local.get 21
        local.get 2
        i32.store8 offset=19
        local.get 21
        local.get 1
        i32.store8 offset=18
        local.get 23
        local.get 21
        i32.const 18
        i32.add
        local.get 22
        i32.add
        i32.load8_u
        local.tee 25
        i32.const 15
        i32.and
        i32.const 65552
        i32.add
        i32.load8_u
        i32.store8
        local.get 23
        i32.const 1
        i32.sub
        local.get 25
        i32.const 4
        i32.shr_u
        i32.const 65552
        i32.add
        i32.load8_u
        i32.store8
        local.get 22
        i32.const 1
        i32.add
        local.set 22
        local.get 23
        i32.const 2
        i32.add
        local.set 23
        br 1 (;@1;)
      end
    end
    local.get 21
    i32.const 8
    i32.add
    local.get 21
    i32.const 38
    i32.add
    i32.const 42
    call $runtime.stringFromBytes
    i32.const 67404
    local.get 24
    i32.store
    local.get 21
    i32.load offset=12
    local.set 1
    local.get 0
    local.get 21
    i32.load offset=8
    i32.store
    local.get 0
    local.get 1
    i32.store offset=4
    local.get 21
    i32.const 96
    i32.add
    global.set $__stack_pointer)
  (func $runtime.stringFromBytes (type 6) (param i32 i32 i32)
    (local i32 i32 i32)
    global.get $__stack_pointer
    i32.const 32
    i32.sub
    local.tee 3
    global.set $__stack_pointer
    local.get 3
    i64.const 0
    i64.store offset=20 align=4
    local.get 3
    i64.const 3
    i64.store offset=12 align=4
    i32.const 67404
    i32.load
    local.set 4
    i32.const 67404
    local.get 3
    i32.const 8
    i32.add
    i32.store
    local.get 3
    local.get 4
    i32.store offset=8
    local.get 2
    i32.const 3
    call $runtime.alloc
    local.tee 5
    local.get 1
    local.get 2
    memory.copy
    i32.const 67404
    local.get 4
    i32.store
    local.get 0
    local.get 2
    i32.store offset=4
    local.get 0
    local.get 5
    i32.store
    local.get 3
    i32.const 32
    i32.add
    global.set $__stack_pointer)
  (func $github.com/weisyn/v1/contracts/sdk/go/framework.AllocateBytes (type 6) (param i32 i32 i32)
    (local i32)
    block  ;; label = @1
      local.get 2
      call $github.com/weisyn/v1/contracts/sdk/go/framework.malloc
      local.tee 3
      i32.eqz
      if  ;; label = @2
        i32.const 0
        local.set 3
        i32.const 0
        local.set 2
        br 1 (;@1;)
      end
      local.get 2
      i32.const 1048576
      i32.le_u
      if  ;; label = @2
        local.get 3
        local.get 1
        local.get 2
        memory.copy
        br 1 (;@1;)
      end
      call $runtime.slicePanic
      unreachable
    end
    local.get 0
    local.get 3
    i32.store
    local.get 0
    local.get 2
    i32.store offset=4)
  (func $runtime.slicePanic (type 7)
    i32.const 66121
    i32.const 18
    call $runtime.runtimePanicAt
    unreachable)
  (func $github.com/weisyn/v1/contracts/sdk/go/framework.Uint64ToString (type 12) (param i32 i64)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const 48
    i32.sub
    local.tee 2
    global.set $__stack_pointer
    local.get 2
    i32.const 4
    i32.store offset=28
    i32.const 67404
    i32.load
    local.set 8
    i32.const 67404
    local.get 2
    i32.const 24
    i32.add
    i32.store
    local.get 2
    local.get 8
    i32.store offset=24
    local.get 2
    i64.const 0
    i64.store offset=40
    block  ;; label = @1
      local.get 1
      i64.eqz
      if  ;; label = @2
        i32.const 1
        local.set 4
        i32.const 65568
        local.set 3
        br 1 (;@1;)
      end
      i32.const 20
      local.set 3
      local.get 2
      i32.const 20
      i32.const 3
      call $runtime.alloc
      local.tee 5
      i32.store offset=32
      loop  ;; label = @2
        local.get 2
        local.get 5
        i32.store offset=36
        local.get 1
        i64.eqz
        if  ;; label = @3
          local.get 4
          local.get 5
          i32.add
          local.set 9
          i32.const 0
          local.set 3
          i32.const -1
          local.set 7
          block  ;; label = @4
            loop  ;; label = @5
              local.get 4
              local.get 7
              i32.add
              local.tee 6
              local.get 3
              i32.gt_s
              if  ;; label = @6
                local.get 3
                local.get 4
                i32.eq
                local.get 4
                local.get 6
                i32.le_u
                i32.or
                br_if 2 (;@4;)
                local.get 3
                local.get 5
                i32.add
                local.tee 6
                i32.load8_u
                local.set 10
                local.get 6
                local.get 7
                local.get 9
                i32.add
                local.tee 6
                i32.load8_u
                i32.store8
                local.get 6
                local.get 10
                i32.store8
                local.get 7
                i32.const 1
                i32.sub
                local.set 7
                local.get 3
                i32.const 1
                i32.add
                local.set 3
                br 1 (;@5;)
              end
            end
            local.get 2
            local.get 5
            local.get 4
            call $runtime.stringFromBytes
            local.get 2
            i32.load offset=4
            local.set 4
            local.get 2
            i32.load
            local.set 3
            br 3 (;@1;)
          end
          call $runtime.lookupPanic
          unreachable
        else
          local.get 2
          local.get 1
          local.get 1
          i64.const 10
          i64.div_u
          local.tee 1
          i64.const 10
          i64.mul
          i64.sub
          i32.wrap_i64
          i32.const 48
          i32.or
          i32.store8 offset=23
          local.get 2
          i32.const 8
          i32.add
          local.get 5
          local.get 2
          i32.const 23
          i32.add
          local.get 4
          local.get 3
          i32.const 1
          call $runtime.sliceAppend
          local.get 2
          local.get 2
          i32.load offset=8
          local.tee 5
          i32.store offset=40
          local.get 2
          i32.load offset=16
          local.set 3
          local.get 2
          i32.load offset=12
          local.set 4
          br 1 (;@2;)
        end
        unreachable
      end
      unreachable
    end
    i32.const 67404
    local.get 8
    i32.store
    local.get 0
    local.get 4
    i32.store offset=4
    local.get 0
    local.get 3
    i32.store
    local.get 2
    i32.const 48
    i32.add
    global.set $__stack_pointer)
  (func $runtime.alloc (type 2) (param i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i32 i32 i64)
    local.get 0
    i32.eqz
    if  ;; label = @1
      i32.const 67400
      return
    end
    i32.const 67376
    i32.const 67376
    i64.load
    i64.const 1
    i64.add
    i64.store
    i32.const 67360
    i32.const 67360
    i64.load
    local.get 0
    i32.const 16
    i32.add
    i64.extend_i32_u
    i64.add
    i64.store
    i32.const 67368
    i32.const 67368
    i64.load
    local.get 0
    i32.const 31
    i32.add
    i32.const 4
    i32.shr_u
    local.tee 5
    i64.extend_i32_u
    i64.add
    i64.store
    i32.const 67352
    i32.load
    local.tee 2
    local.set 4
    loop  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 2
            local.get 4
            i32.ne
            br_if 0 (;@4;)
            local.get 3
            i32.const 255
            i32.and
            local.set 2
            i32.const 1
            local.set 3
            block  ;; label = @5
              block  ;; label = @6
                local.get 2
                br_table 2 (;@4;) 0 (;@6;) 1 (;@5;)
              end
              i32.const 67404
              i32.load
              drop
              global.get $__stack_pointer
              i32.const 65536
              call $runtime.markRoots
              i32.const 65536
              i32.const 67696
              call $runtime.markRoots
              loop  ;; label = @6
                i32.const 67401
                i32.load8_u
                i32.eqz
                if  ;; label = @7
                  i64.const 0
                  local.set 9
                  i32.const 0
                  local.set 6
                  i32.const 0
                  local.set 3
                  i32.const 0
                  local.set 2
                  loop  ;; label = @8
                    block  ;; label = @9
                      block  ;; label = @10
                        i32.const 67356
                        i32.load
                        local.get 2
                        i32.gt_u
                        if  ;; label = @11
                          block  ;; label = @12
                            block  ;; label = @13
                              block  ;; label = @14
                                block  ;; label = @15
                                  local.get 2
                                  call $_runtime.gcBlock_.state
                                  i32.const 255
                                  i32.and
                                  i32.const 1
                                  i32.sub
                                  br_table 0 (;@15;) 1 (;@14;) 2 (;@13;) 3 (;@12;)
                                end
                                local.get 2
                                call $_runtime.gcBlock_.markFree
                                i32.const 67384
                                i32.const 67384
                                i64.load
                                i64.const 1
                                i64.add
                                i64.store
                                br 4 (;@10;)
                              end
                              local.get 3
                              i32.const 1
                              i32.and
                              i32.const 0
                              local.set 3
                              i32.eqz
                              br_if 4 (;@9;)
                              local.get 2
                              call $_runtime.gcBlock_.markFree
                              br 3 (;@10;)
                            end
                            i32.const 0
                            local.set 3
                            i32.const 67348
                            i32.load
                            local.get 2
                            i32.const 2
                            i32.shr_u
                            i32.add
                            local.tee 7
                            local.get 7
                            i32.load8_u
                            i32.const 2
                            local.get 2
                            i32.const 1
                            i32.shl
                            i32.const 6
                            i32.and
                            i32.shl
                            i32.const -1
                            i32.xor
                            i32.and
                            i32.store8
                            br 3 (;@9;)
                          end
                          local.get 6
                          i32.const 16
                          i32.add
                          local.set 6
                          br 2 (;@9;)
                        end
                        i32.const 67392
                        i32.const 67392
                        i64.load
                        local.get 9
                        i64.add
                        i64.store
                        i32.const 2
                        local.set 3
                        local.get 9
                        i32.wrap_i64
                        i32.const 4
                        i32.shl
                        local.get 6
                        i32.add
                        i32.const 67348
                        i32.load
                        i32.const 67696
                        i32.sub
                        i32.const 3
                        i32.div_u
                        i32.ge_u
                        br_if 6 (;@4;)
                        call $runtime.growHeap
                        drop
                        br 6 (;@4;)
                      end
                      local.get 9
                      i64.const 1
                      i64.add
                      local.set 9
                      i32.const 1
                      local.set 3
                    end
                    local.get 2
                    i32.const 1
                    i32.add
                    local.set 2
                    br 0 (;@8;)
                  end
                  unreachable
                end
                i32.const 0
                local.set 2
                i32.const 67401
                i32.const 0
                i32.store8
                i32.const 67356
                i32.load
                local.set 3
                loop  ;; label = @7
                  local.get 2
                  local.get 3
                  i32.ge_u
                  br_if 1 (;@6;)
                  local.get 2
                  call $_runtime.gcBlock_.state
                  i32.const 255
                  i32.and
                  i32.const 3
                  i32.eq
                  if  ;; label = @8
                    local.get 2
                    call $runtime.startMark
                    i32.const 67356
                    i32.load
                    local.set 3
                  end
                  local.get 2
                  i32.const 1
                  i32.add
                  local.set 2
                  br 0 (;@7;)
                end
                unreachable
              end
              unreachable
            end
            call $runtime.growHeap
            i32.const 1
            i32.and
            i32.eqz
            br_if 1 (;@3;)
            i32.const 2
            local.set 3
          end
          block  ;; label = @4
            i32.const 67356
            i32.load
            local.get 4
            i32.eq
            if (result i32)  ;; label = @5
              i32.const 0
            else
              local.get 4
              call $_runtime.gcBlock_.state
              i32.const 255
              i32.and
              i32.eqz
              br_if 1 (;@4;)
              local.get 4
              i32.const 1
              i32.add
            end
            local.set 4
            i32.const 0
            local.set 8
            br 2 (;@2;)
          end
          local.get 4
          i32.const 1
          i32.add
          local.set 2
          local.get 5
          local.get 8
          i32.const 1
          i32.add
          local.tee 8
          i32.ne
          if  ;; label = @4
            local.get 2
            local.set 4
            br 2 (;@2;)
          end
          i32.const 67352
          local.get 2
          i32.store
          local.get 2
          local.get 5
          i32.sub
          local.tee 3
          i32.const 1
          call $_runtime.gcBlock_.setState
          local.get 4
          local.get 5
          i32.sub
          i32.const 2
          i32.add
          local.set 2
          loop  ;; label = @4
            i32.const 67352
            i32.load
            local.get 2
            i32.ne
            if  ;; label = @5
              local.get 2
              i32.const 2
              call $_runtime.gcBlock_.setState
              local.get 2
              i32.const 1
              i32.add
              local.set 2
              br 1 (;@4;)
            end
          end
          local.get 3
          i32.const 4
          i32.shl
          i32.const 67696
          i32.add
          local.tee 4
          local.get 1
          i32.store
          local.get 4
          i32.const 16
          i32.add
          local.tee 1
          i32.const 0
          local.get 0
          memory.fill
          local.get 1
          return
        end
        i32.const 66008
        i32.const 13
        call $runtime.runtimePanicAt
        unreachable
      end
      i32.const 67352
      i32.load
      local.set 2
      br 0 (;@1;)
    end
    unreachable)
  (func $runtime.lookupPanic (type 7)
    i32.const 66103
    i32.const 18
    call $runtime.runtimePanicAt
    unreachable)
  (func $runtime.sliceAppend (type 13) (param i32 i32 i32 i32 i32 i32)
    (local i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const 32
    i32.sub
    local.tee 6
    global.set $__stack_pointer
    local.get 6
    i32.const 28
    i32.add
    i32.const 0
    i32.store
    local.get 6
    i64.const 0
    i64.store offset=20 align=4
    local.get 6
    i32.const 4
    i32.store offset=12
    i32.const 67404
    i32.load
    local.set 8
    i32.const 67404
    local.get 6
    i32.const 8
    i32.add
    i32.store
    local.get 6
    local.get 8
    i32.store offset=8
    block  ;; label = @1
      local.get 4
      local.get 3
      i32.const 1
      i32.add
      local.tee 9
      i32.ge_u
      if  ;; label = @2
        local.get 1
        local.set 7
        br 1 (;@1;)
      end
      local.get 6
      i32.const 3
      i32.const 0
      local.get 5
      i32.const 4
      i32.lt_u
      select
      local.tee 7
      i32.store offset=16
      i32.const 1
      i32.const 32
      local.get 9
      i32.clz
      local.tee 4
      i32.sub
      i32.shl
      i32.const 0
      local.get 4
      select
      local.tee 4
      local.get 5
      i32.mul
      local.get 7
      call $runtime.alloc
      local.set 7
      local.get 3
      i32.eqz
      br_if 0 (;@1;)
      local.get 7
      local.get 1
      local.get 3
      local.get 5
      i32.mul
      memory.copy
    end
    local.get 7
    local.get 3
    local.get 5
    i32.mul
    i32.add
    local.get 2
    local.get 5
    memory.copy
    i32.const 67404
    local.get 8
    i32.store
    local.get 0
    local.get 4
    i32.store offset=8
    local.get 0
    local.get 9
    i32.store offset=4
    local.get 0
    local.get 7
    i32.store
    local.get 6
    i32.const 32
    i32.add
    global.set $__stack_pointer)
  (func $_*github.com/weisyn/v1/contracts/sdk/go/framework.ContractParams_.ParseJSON (type 8) (param i32 i32 i32 i32)
    (local i32 i32 i32 i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const 48
    i32.sub
    local.tee 4
    global.set $__stack_pointer
    local.get 4
    i32.const 44
    i32.add
    i32.const 0
    i32.store
    local.get 4
    i64.const 0
    i64.store offset=36 align=4
    local.get 4
    i32.const 4
    i32.store offset=28
    i32.const 67404
    i32.load
    local.set 10
    i32.const 67404
    local.get 4
    i32.const 24
    i32.add
    i32.store
    local.get 4
    local.get 10
    i32.store offset=24
    block  ;; label = @1
      local.get 1
      if  ;; label = @2
        local.get 4
        local.get 1
        i32.load
        local.tee 6
        i32.store offset=32
        local.get 4
        i32.const 16
        i32.add
        local.get 6
        local.get 1
        i32.load offset=4
        call $runtime.stringFromBytes
        local.get 4
        local.get 4
        i32.load offset=16
        local.tee 9
        i32.store offset=36
        local.get 4
        i32.load offset=20
        local.set 7
        local.get 4
        i32.const 8
        i32.add
        i32.const 65922
        i32.const 1
        local.get 2
        local.get 3
        call $runtime.stringConcat
        local.get 4
        local.get 4
        i32.load offset=8
        local.tee 1
        i32.store offset=40
        local.get 4
        local.get 1
        local.get 4
        i32.load offset=12
        i32.const 65569
        i32.const 3
        call $runtime.stringConcat
        local.get 4
        local.get 4
        i32.load
        local.tee 1
        i32.store offset=44
        local.get 7
        local.get 4
        i32.load offset=4
        local.tee 6
        i32.sub
        local.set 2
        block  ;; label = @3
          loop  ;; label = @4
            local.get 2
            local.get 5
            i32.lt_s
            if  ;; label = @5
              i32.const 0
              local.set 5
              i32.const 0
              local.set 3
              br 4 (;@1;)
            end
            local.get 5
            local.get 6
            i32.add
            local.tee 3
            local.get 6
            i32.lt_u
            local.get 3
            local.get 7
            i32.gt_u
            i32.or
            br_if 1 (;@3;)
            local.get 5
            local.get 9
            i32.add
            local.get 5
            i32.const 1
            i32.add
            local.tee 8
            local.set 5
            local.get 6
            local.get 1
            local.get 6
            call $runtime.stringEqual
            i32.const 1
            i32.and
            i32.eqz
            br_if 0 (;@4;)
          end
          i32.const 0
          local.set 5
          i32.const 0
          local.set 3
          i32.const 0
          local.get 6
          i32.sub
          local.get 8
          i32.eq
          br_if 2 (;@1;)
          local.get 7
          local.get 6
          local.get 8
          i32.add
          i32.const 1
          i32.sub
          local.tee 2
          local.get 2
          local.get 7
          i32.lt_s
          select
          local.set 1
          local.get 2
          local.set 5
          loop  ;; label = @4
            block  ;; label = @5
              local.get 5
              local.get 7
              i32.lt_s
              if  ;; label = @6
                local.get 5
                local.get 9
                i32.add
                i32.load8_u
                i32.const 34
                i32.ne
                br_if 1 (;@5;)
                local.get 5
                local.set 1
              end
              i32.const 0
              local.set 5
              local.get 1
              local.get 2
              i32.le_s
              br_if 4 (;@1;)
              local.get 1
              local.get 2
              i32.lt_u
              local.get 1
              local.get 7
              i32.gt_u
              i32.or
              br_if 2 (;@3;)
              local.get 1
              local.get 6
              i32.sub
              local.get 8
              i32.sub
              i32.const 1
              i32.add
              local.set 3
              local.get 6
              local.get 9
              i32.add
              local.get 8
              i32.add
              i32.const 1
              i32.sub
              local.set 5
              br 4 (;@1;)
            end
            local.get 5
            i32.const 1
            i32.add
            local.set 5
            br 0 (;@4;)
          end
          unreachable
        end
        call $runtime.slicePanic
        unreachable
      end
      call $runtime.nilPanic
      unreachable
    end
    i32.const 67404
    local.get 10
    i32.store
    local.get 0
    local.get 3
    i32.store offset=4
    local.get 0
    local.get 5
    i32.store
    local.get 4
    i32.const 48
    i32.add
    global.set $__stack_pointer)
  (func $runtime.stringConcat (type 14) (param i32 i32 i32 i32 i32)
    (local i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const 32
    i32.sub
    local.tee 5
    global.set $__stack_pointer
    local.get 5
    i64.const 0
    i64.store offset=20 align=4
    local.get 5
    i64.const 3
    i64.store offset=12 align=4
    i32.const 67404
    i32.load
    local.set 8
    i32.const 67404
    local.get 5
    i32.const 8
    i32.add
    i32.store
    local.get 5
    local.get 8
    i32.store offset=8
    block  ;; label = @1
      local.get 2
      i32.eqz
      if  ;; label = @2
        local.get 3
        local.set 6
        local.get 4
        local.set 7
        br 1 (;@1;)
      end
      local.get 4
      i32.eqz
      if  ;; label = @2
        local.get 1
        local.set 6
        local.get 2
        local.set 7
        br 1 (;@1;)
      end
      local.get 2
      local.get 4
      i32.add
      local.tee 7
      i32.const 3
      call $runtime.alloc
      local.tee 6
      local.get 1
      local.get 2
      memory.copy
      local.get 2
      local.get 6
      i32.add
      local.get 3
      local.get 4
      memory.copy
    end
    i32.const 67404
    local.get 8
    i32.store
    local.get 0
    local.get 7
    i32.store offset=4
    local.get 0
    local.get 6
    i32.store
    local.get 5
    i32.const 32
    i32.add
    global.set $__stack_pointer)
  (func $runtime.stringEqual (type 0) (param i32 i32 i32 i32) (result i32)
    (local i32 i32)
    block  ;; label = @1
      local.get 1
      local.get 3
      i32.ne
      br_if 0 (;@1;)
      local.get 1
      i32.const 0
      local.get 1
      i32.const 0
      i32.gt_s
      select
      local.set 1
      loop  ;; label = @2
        local.get 1
        i32.eqz
        local.set 4
        local.get 1
        i32.eqz
        br_if 1 (;@1;)
        local.get 1
        i32.const 1
        i32.sub
        local.set 1
        local.get 2
        i32.load8_u
        local.get 0
        i32.load8_u
        local.get 2
        i32.const 1
        i32.add
        local.set 2
        local.get 0
        i32.const 1
        i32.add
        local.set 0
        i32.eq
        br_if 0 (;@2;)
      end
    end
    local.get 4)
  (func $runtime.nilPanic (type 7)
    i32.const 66050
    i32.const 23
    call $runtime.runtimePanicAt
    unreachable)
  (func $github.com/weisyn/v1/contracts/sdk/go/framework.NewContractParams (type 10) (param i32 i32 i32) (result i32)
    local.get 0
    local.get 1
    local.get 2
    i32.const 71
    call 68)
  (func $github.com/weisyn/v1/contracts/sdk/go/framework.NewContractError (type 10) (param i32 i32 i32) (result i32)
    local.get 0
    local.get 1
    local.get 2
    i32.const 135
    call 68)
  (func $github.com/weisyn/v1/contracts/sdk/go/framework.hexCharToNibble (type 4) (param i32) (result i32)
    (local i32)
    local.get 0
    i32.const 48
    i32.sub
    local.tee 1
    i32.const 255
    i32.and
    i32.const 9
    i32.gt_u
    if (result i32)  ;; label = @1
      local.get 0
      i32.const 97
      i32.sub
      i32.const 255
      i32.and
      i32.const 5
      i32.le_u
      if  ;; label = @2
        local.get 0
        i32.const 87
        i32.sub
        return
      end
      i32.const -1
      local.get 0
      i32.const 55
      i32.sub
      local.get 0
      i32.const 65
      i32.sub
      i32.const 255
      i32.and
      i32.const 6
      i32.ge_u
      select
    else
      local.get 1
    end)
  (func $github.com/weisyn/v1/contracts/sdk/go/framework.GetCaller (type 1) (param i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32)
    local.get 0
    block (result i32)  ;; label = @1
      i32.const 20
      call $github.com/weisyn/v1/contracts/sdk/go/framework.malloc
      local.tee 1
      i32.eqz
      if  ;; label = @2
        i32.const 0
        br 1 (;@1;)
      end
      local.get 1
      call $github.com/weisyn/v1/contracts/sdk/go/framework.getCaller
      drop
      local.get 1
      i32.load8_u offset=19
      local.set 20
      local.get 1
      i32.load8_u offset=18
      local.set 19
      local.get 1
      i32.load8_u offset=17
      local.set 18
      local.get 1
      i32.load8_u offset=16
      local.set 17
      local.get 1
      i32.load8_u offset=15
      local.set 16
      local.get 1
      i32.load8_u offset=14
      local.set 15
      local.get 1
      i32.load8_u offset=13
      local.set 14
      local.get 1
      i32.load8_u offset=12
      local.set 13
      local.get 1
      i32.load8_u offset=11
      local.set 12
      local.get 1
      i32.load8_u offset=10
      local.set 11
      local.get 1
      i32.load8_u offset=9
      local.set 10
      local.get 1
      i32.load8_u offset=8
      local.set 9
      local.get 1
      i32.load8_u offset=7
      local.set 8
      local.get 1
      i32.load8_u offset=6
      local.set 7
      local.get 1
      i32.load8_u offset=5
      local.set 6
      local.get 1
      i32.load8_u offset=4
      local.set 5
      local.get 1
      i32.load8_u offset=3
      local.set 4
      local.get 1
      i32.load8_u offset=2
      local.set 3
      local.get 1
      i32.load8_u offset=1
      local.set 2
      local.get 1
      i32.load8_u
    end
    i32.store8
    local.get 0
    local.get 2
    i32.store8 offset=1
    local.get 0
    local.get 3
    i32.store8 offset=2
    local.get 0
    local.get 4
    i32.store8 offset=3
    local.get 0
    local.get 5
    i32.store8 offset=4
    local.get 0
    local.get 6
    i32.store8 offset=5
    local.get 0
    local.get 7
    i32.store8 offset=6
    local.get 0
    local.get 8
    i32.store8 offset=7
    local.get 0
    local.get 9
    i32.store8 offset=8
    local.get 0
    local.get 10
    i32.store8 offset=9
    local.get 0
    local.get 11
    i32.store8 offset=10
    local.get 0
    local.get 12
    i32.store8 offset=11
    local.get 0
    local.get 13
    i32.store8 offset=12
    local.get 0
    local.get 14
    i32.store8 offset=13
    local.get 0
    local.get 15
    i32.store8 offset=14
    local.get 0
    local.get 16
    i32.store8 offset=15
    local.get 0
    local.get 17
    i32.store8 offset=16
    local.get 0
    local.get 18
    i32.store8 offset=17
    local.get 0
    local.get 19
    i32.store8 offset=18
    local.get 0
    local.get 20
    i32.store8 offset=19)
  (func $github.com/weisyn/v1/contracts/sdk/go/framework.SetReturnString (type 6) (param i32 i32 i32)
    (local i32 i32 i32)
    global.get $__stack_pointer
    i32.const 32
    i32.sub
    local.tee 3
    global.set $__stack_pointer
    local.get 3
    i32.const 24
    i32.add
    i64.const 0
    i64.store
    local.get 3
    i64.const 0
    i64.store offset=16
    local.get 3
    i32.const 4
    i32.store offset=12
    i32.const 67404
    i32.load
    local.set 5
    i32.const 67404
    local.get 3
    i32.const 8
    i32.add
    i32.store
    local.get 3
    local.get 5
    i32.store offset=8
    local.get 3
    local.get 1
    local.get 2
    call $github.com/weisyn/v1/contracts/sdk/go/framework.AllocateBytes
    block  ;; label = @1
      block (result i32)  ;; label = @2
        local.get 3
        i32.load
        local.tee 1
        i32.eqz
        if  ;; label = @3
          i32.const 6
          i32.const 65818
          i32.const 30
          call $github.com/weisyn/v1/contracts/sdk/go/framework.NewContractError
          br 1 (;@2;)
        end
        local.get 1
        local.get 3
        i32.load offset=4
        call $github.com/weisyn/v1/contracts/sdk/go/framework.setReturnData
        local.tee 1
        i32.eqz
        if  ;; label = @3
          i32.const 0
          local.set 2
          br 2 (;@1;)
        end
        local.get 1
        i32.const 65848
        i32.const 25
        call $github.com/weisyn/v1/contracts/sdk/go/framework.NewContractError
      end
      local.set 2
      i32.const 66600
      local.set 4
    end
    i32.const 67404
    local.get 5
    i32.store
    local.get 0
    local.get 2
    i32.store offset=4
    local.get 0
    local.get 4
    i32.store
    local.get 3
    i32.const 32
    i32.add
    global.set $__stack_pointer)
  (func $github.com/weisyn/v1/contracts/sdk/go/framework.SetReturnJSON (type 4) (param i32) (result i32)
    (local i32 i32 i32)
    global.get $__stack_pointer
    i32.const 48
    i32.sub
    local.tee 1
    global.set $__stack_pointer
    local.get 1
    i32.const 36
    i32.add
    i64.const 0
    i64.store align=4
    local.get 1
    i64.const 0
    i64.store offset=28 align=4
    local.get 1
    i32.const 5
    i32.store offset=20
    i32.const 67404
    i32.load
    local.set 2
    i32.const 67404
    local.get 1
    i32.const 16
    i32.add
    i32.store
    local.get 1
    local.get 2
    i32.store offset=16
    local.get 1
    i32.const 8
    i32.add
    i32.const 66300
    local.get 0
    call $github.com/weisyn/v1/contracts/sdk/go/framework.serializeToJSON
    local.get 1
    local.get 1
    i32.load offset=8
    local.tee 0
    i32.store offset=24
    block (result i32)  ;; label = @1
      local.get 1
      i32.load offset=12
      local.tee 3
      i32.eqz
      if  ;; label = @2
        i32.const 1
        i32.const 65873
        i32.const 23
        call $github.com/weisyn/v1/contracts/sdk/go/framework.NewContractError
        drop
        i32.const 66600
        br 1 (;@1;)
      end
      local.get 1
      local.get 0
      local.get 3
      call $github.com/weisyn/v1/contracts/sdk/go/framework.SetReturnString
      local.get 1
      i32.load
    end
    i32.const 67404
    local.get 2
    i32.store
    local.get 1
    i32.const 48
    i32.add
    global.set $__stack_pointer)
  (func $github.com/weisyn/v1/contracts/sdk/go/framework.serializeToJSON (type 6) (param i32 i32 i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i64)
    global.get $__stack_pointer
    i32.const 368
    i32.sub
    local.tee 3
    global.set $__stack_pointer
    local.get 3
    i32.const 33
    i32.store offset=228
    local.get 3
    i32.const 232
    i32.add
    i32.const 0
    i32.const 132
    memory.fill
    local.get 3
    i32.const 67404
    i32.load
    local.tee 10
    i32.store offset=224
    i32.const 67404
    local.get 3
    i32.const 224
    i32.add
    i32.store
    block  ;; label = @1
      block  ;; label = @2
        local.get 1
        i32.const 66216
        i32.eq
        if  ;; label = @3
          local.get 3
          i32.const 16
          i32.add
          local.get 2
          i32.load
          local.get 2
          i32.load offset=4
          call $github.com/weisyn/v1/contracts/sdk/go/framework.escapeJSONString
          local.get 3
          local.get 3
          i32.load offset=16
          local.tee 1
          i32.store offset=236
          local.get 3
          i32.const 8
          i32.add
          i32.const 65922
          i32.const 1
          local.get 1
          local.get 3
          i32.load offset=20
          call $runtime.stringConcat
          local.get 3
          local.get 3
          i32.load offset=8
          local.tee 1
          i32.store offset=240
          local.get 3
          local.get 1
          local.get 3
          i32.load offset=12
          i32.const 65922
          i32.const 1
          call $runtime.stringConcat
          local.get 3
          i32.load offset=4
          local.set 4
          local.get 3
          i32.load
          local.set 5
          br 1 (;@2;)
        end
        local.get 1
        i32.const 66188
        i32.eq
        if  ;; label = @3
          local.get 3
          i32.const 24
          i32.add
          local.get 2
          i64.load
          call $github.com/weisyn/v1/contracts/sdk/go/framework.Uint64ToString
          local.get 3
          i32.load offset=28
          local.set 4
          local.get 3
          i32.load offset=24
          local.set 5
          br 1 (;@2;)
        end
        local.get 1
        i32.const 65900
        i32.eq
        if  ;; label = @3
          local.get 2
          i64.load
          local.tee 11
          i64.const 0
          i64.lt_s
          if  ;; label = @4
            local.get 3
            i32.const 40
            i32.add
            i64.const 0
            local.get 11
            i64.sub
            call $github.com/weisyn/v1/contracts/sdk/go/framework.Uint64ToString
            local.get 3
            local.get 3
            i32.load offset=40
            local.tee 1
            i32.store offset=244
            local.get 3
            i32.const 32
            i32.add
            i32.const 65896
            i32.const 1
            local.get 1
            local.get 3
            i32.load offset=44
            call $runtime.stringConcat
            local.get 3
            i32.load offset=36
            local.set 4
            local.get 3
            i32.load offset=32
            local.set 5
            br 2 (;@2;)
          end
          local.get 3
          i32.const 48
          i32.add
          local.get 11
          call $github.com/weisyn/v1/contracts/sdk/go/framework.Uint64ToString
          local.get 3
          i32.load offset=52
          local.set 4
          local.get 3
          i32.load offset=48
          local.set 5
          br 1 (;@2;)
        end
        local.get 1
        i32.const 65712
        i32.eq
        if  ;; label = @3
          local.get 3
          i32.const 56
          i32.add
          local.get 2
          i64.extend_i32_u
          call $github.com/weisyn/v1/contracts/sdk/go/framework.Uint64ToString
          local.get 3
          i32.load offset=60
          local.set 4
          local.get 3
          i32.load offset=56
          local.set 5
          br 1 (;@2;)
        end
        local.get 1
        i32.eqz
        if  ;; label = @3
          i32.const 4
          local.set 4
          i32.const 65916
          local.set 5
          br 1 (;@2;)
        end
        local.get 1
        i32.const 66300
        i32.eq
        if  ;; label = @3
          i32.const 2
          local.set 4
          i32.const 65920
          local.set 5
          local.get 2
          i32.eqz
          br_if 1 (;@2;)
          local.get 2
          i32.load offset=8
          local.tee 7
          i32.eqz
          br_if 1 (;@2;)
          local.get 7
          i32.const 536870911
          i32.gt_u
          br_if 2 (;@1;)
          local.get 3
          local.get 7
          i32.const 3
          i32.shl
          i32.const 69
          call $runtime.alloc
          local.tee 4
          i32.store offset=248
          local.get 3
          i32.const 191
          i32.add
          i64.const 0
          i64.store align=1
          local.get 3
          i32.const 184
          i32.add
          i64.const 0
          i64.store
          local.get 3
          i64.const 0
          i64.store offset=176
          loop  ;; label = @4
            local.get 3
            local.get 4
            i32.store offset=252
            block  ;; label = @5
              loop  ;; label = @6
                local.get 2
                local.get 3
                i32.const 176
                i32.add
                local.get 3
                i32.const 200
                i32.add
                local.get 3
                i32.const 208
                i32.add
                call $runtime.hashmapNext
                local.get 3
                local.get 3
                i32.load offset=200
                local.tee 5
                i32.store offset=256
                local.get 3
                local.get 3
                i32.load offset=208
                local.tee 6
                i32.store offset=260
                local.get 3
                local.get 3
                i32.load offset=212
                local.tee 9
                i32.store offset=264
                i32.const 1
                i32.and
                i32.eqz
                br_if 1 (;@5;)
                local.get 3
                i32.load offset=204
                local.set 1
                local.get 3
                i32.const 112
                i32.add
                local.get 6
                local.get 9
                call $github.com/weisyn/v1/contracts/sdk/go/framework.serializeToJSON
                local.get 3
                local.get 3
                i32.load offset=112
                local.tee 6
                i32.store offset=268
                local.get 3
                i32.load offset=116
                local.tee 9
                i32.eqz
                br_if 0 (;@6;)
              end
              local.get 3
              i32.const 104
              i32.add
              local.get 5
              local.get 1
              call $github.com/weisyn/v1/contracts/sdk/go/framework.escapeJSONString
              local.get 3
              local.get 3
              i32.load offset=104
              local.tee 1
              i32.store offset=272
              local.get 3
              i32.const 96
              i32.add
              i32.const 65922
              i32.const 1
              local.get 1
              local.get 3
              i32.load offset=108
              call $runtime.stringConcat
              local.get 3
              local.get 3
              i32.load offset=96
              local.tee 1
              i32.store offset=276
              local.get 3
              i32.const 88
              i32.add
              local.get 1
              local.get 3
              i32.load offset=100
              i32.const 65923
              i32.const 2
              call $runtime.stringConcat
              local.get 3
              local.get 3
              i32.load offset=88
              local.tee 1
              i32.store offset=280
              local.get 3
              i32.const 80
              i32.add
              local.get 1
              local.get 3
              i32.load offset=92
              local.get 6
              local.get 9
              call $runtime.stringConcat
              local.get 3
              local.get 3
              i32.load offset=80
              local.tee 1
              i32.store offset=284
              local.get 3
              local.get 3
              i32.load offset=84
              i32.store offset=220
              local.get 3
              local.get 1
              i32.store offset=216
              local.get 3
              i32.const -64
              i32.sub
              local.get 4
              local.get 3
              i32.const 216
              i32.add
              local.get 8
              local.get 7
              i32.const 8
              call $runtime.sliceAppend
              local.get 3
              local.get 3
              i32.load offset=64
              local.tee 4
              i32.store offset=288
              local.get 3
              i32.load offset=72
              local.set 7
              local.get 3
              i32.load offset=68
              local.set 8
              br 1 (;@4;)
            end
          end
          i32.const 0
          local.set 2
          local.get 8
          i32.const 0
          local.get 8
          i32.const 0
          i32.gt_s
          select
          local.set 7
          i32.const 65926
          local.set 1
          i32.const 1
          local.set 5
          block  ;; label = @4
            loop  ;; label = @5
              local.get 3
              local.get 1
              i32.store offset=292
              local.get 2
              local.get 7
              i32.eq
              br_if 1 (;@4;)
              local.get 2
              local.get 8
              i32.ne
              if  ;; label = @6
                local.get 3
                local.get 4
                i32.load
                local.tee 6
                i32.store offset=296
                local.get 4
                i32.const 4
                i32.add
                i32.load
                local.set 9
                local.get 2
                i32.const 1
                i32.sub
                i32.const 2147483646
                i32.le_u
                if  ;; label = @7
                  local.get 3
                  i32.const 136
                  i32.add
                  local.get 1
                  local.get 5
                  i32.const 65929
                  i32.const 1
                  call $runtime.stringConcat
                  local.get 3
                  local.get 3
                  i32.load offset=136
                  local.tee 1
                  i32.store offset=300
                  local.get 3
                  i32.load offset=140
                  local.set 5
                end
                local.get 3
                local.get 1
                i32.store offset=304
                local.get 3
                i32.const 128
                i32.add
                local.get 1
                local.get 5
                local.get 6
                local.get 9
                call $runtime.stringConcat
                local.get 3
                local.get 3
                i32.load offset=128
                local.tee 1
                i32.store offset=308
                local.get 4
                i32.const 8
                i32.add
                local.set 4
                local.get 2
                i32.const 1
                i32.add
                local.set 2
                local.get 3
                i32.load offset=132
                local.set 5
                br 1 (;@5;)
              end
            end
            call $runtime.lookupPanic
            unreachable
          end
          local.get 3
          i32.const 120
          i32.add
          local.get 1
          local.get 5
          i32.const 65925
          i32.const 1
          call $runtime.stringConcat
          local.get 3
          i32.load offset=124
          local.set 4
          local.get 3
          i32.load offset=120
          local.set 5
          br 1 (;@2;)
        end
        local.get 1
        i32.const 66568
        i32.ne
        br_if 0 (;@2;)
        local.get 2
        i32.load offset=4
        local.tee 9
        i32.const 536870911
        i32.gt_u
        br_if 1 (;@1;)
        local.get 2
        i32.load
        local.set 2
        local.get 3
        local.get 9
        i32.const 3
        i32.shl
        i32.const 197
        call $runtime.alloc
        local.tee 6
        i32.store offset=316
        local.get 9
        local.set 5
        local.get 6
        local.set 4
        loop  ;; label = @3
          local.get 5
          if  ;; label = @4
            local.get 3
            local.get 2
            i32.load
            local.tee 7
            i32.store offset=320
            local.get 2
            i32.const 4
            i32.add
            i32.load
            local.set 1
            i32.const 8
            i32.const 0
            call $runtime.alloc
            local.tee 8
            local.get 1
            i32.store offset=4
            local.get 8
            local.get 7
            i32.store
            local.get 4
            i32.const 4
            i32.add
            local.get 8
            i32.store
            local.get 4
            i32.const 66216
            i32.store
            local.get 3
            local.get 8
            i32.store offset=324
            local.get 3
            local.get 8
            i32.store offset=328
            local.get 5
            i32.const 1
            i32.sub
            local.set 5
            local.get 2
            i32.const 8
            i32.add
            local.set 2
            local.get 4
            i32.const 8
            i32.add
            local.set 4
            br 1 (;@3;)
          end
        end
        local.get 9
        i32.eqz
        if  ;; label = @3
          i32.const 2
          local.set 4
          i32.const 65927
          local.set 5
          br 1 (;@2;)
        end
        i32.const 65931
        local.set 1
        i32.const 0
        local.set 2
        i32.const 1
        local.set 4
        loop  ;; label = @3
          local.get 3
          local.get 1
          i32.store offset=332
          local.get 2
          local.get 9
          i32.eq
          i32.eqz
          if  ;; label = @4
            local.get 3
            local.get 6
            i32.load
            local.tee 7
            i32.store offset=336
            local.get 3
            local.get 6
            i32.const 4
            i32.add
            i32.load
            local.tee 5
            i32.store offset=340
            local.get 2
            i32.const 1
            i32.sub
            i32.const 2147483646
            i32.le_u
            if  ;; label = @5
              local.get 3
              i32.const 168
              i32.add
              local.get 1
              local.get 4
              i32.const 65929
              i32.const 1
              call $runtime.stringConcat
              local.get 3
              local.get 3
              i32.load offset=168
              local.tee 1
              i32.store offset=344
              local.get 3
              i32.load offset=172
              local.set 4
            end
            local.get 3
            local.get 1
            i32.store offset=348
            local.get 3
            i32.const 160
            i32.add
            local.get 7
            local.get 5
            call $github.com/weisyn/v1/contracts/sdk/go/framework.serializeToJSON
            local.get 3
            local.get 3
            i32.load offset=160
            local.tee 5
            i32.store offset=352
            local.get 3
            i32.const 152
            i32.add
            local.get 1
            local.get 4
            local.get 5
            local.get 3
            i32.load offset=164
            call $runtime.stringConcat
            local.get 3
            local.get 3
            i32.load offset=152
            local.tee 1
            i32.store offset=356
            local.get 6
            i32.const 8
            i32.add
            local.set 6
            local.get 2
            i32.const 1
            i32.add
            local.set 2
            local.get 3
            i32.load offset=156
            local.set 4
            br 1 (;@3;)
          end
        end
        local.get 3
        i32.const 144
        i32.add
        local.get 1
        local.get 4
        i32.const 65930
        i32.const 1
        call $runtime.stringConcat
        local.get 3
        i32.load offset=148
        local.set 4
        local.get 3
        i32.load offset=144
        local.set 5
      end
      i32.const 67404
      local.get 10
      i32.store
      local.get 0
      local.get 4
      i32.store offset=4
      local.get 0
      local.get 5
      i32.store
      local.get 3
      i32.const 368
      i32.add
      global.set $__stack_pointer
      return
    end
    call $runtime.slicePanic
    unreachable)
  (func $github.com/weisyn/v1/contracts/sdk/go/framework.escapeJSONString (type 6) (param i32 i32 i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const 96
    i32.sub
    local.tee 3
    global.set $__stack_pointer
    local.get 3
    i32.const 76
    i32.add
    i64.const 0
    i64.store align=4
    local.get 3
    i32.const 84
    i32.add
    i64.const 0
    i64.store align=4
    local.get 3
    i64.const 0
    i64.store offset=68 align=4
    local.get 3
    i32.const 7
    i32.store offset=60
    i32.const 67404
    i32.load
    local.set 13
    i32.const 67404
    local.get 3
    i32.const 56
    i32.add
    i32.store
    local.get 3
    local.get 13
    i32.store offset=56
    loop  ;; label = @1
      local.get 3
      local.get 10
      i32.store offset=64
      local.get 3
      block (result i32)  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            block (result i32)  ;; label = @5
              local.get 2
              local.get 11
              i32.le_s
              if  ;; label = @6
                i32.const 0
                local.set 4
                i32.const 0
                br 1 (;@5;)
              end
              block  ;; label = @6
                local.get 1
                local.get 11
                i32.add
                local.tee 6
                i32.load8_s
                local.tee 5
                i32.const 0
                i32.ge_s
                if  ;; label = @7
                  i32.const 1
                  local.set 7
                  local.get 5
                  local.set 4
                  br 1 (;@6;)
                end
                local.get 2
                local.get 11
                i32.sub
                local.set 8
                block  ;; label = @7
                  local.get 5
                  i32.const -32
                  i32.and
                  i32.const -64
                  i32.eq
                  if  ;; label = @8
                    i32.const 65533
                    local.set 4
                    local.get 8
                    i32.const 2
                    i32.lt_u
                    br_if 1 (;@7;)
                    i32.const 1
                    local.set 7
                    local.get 6
                    i32.const 1
                    i32.add
                    i32.load8_u
                    local.tee 6
                    i32.const 192
                    i32.and
                    i32.const 128
                    i32.ne
                    br_if 2 (;@6;)
                    local.get 5
                    i32.const 31
                    i32.and
                    local.tee 5
                    i32.const 2
                    i32.lt_u
                    br_if 2 (;@6;)
                    local.get 6
                    i32.const 63
                    i32.and
                    local.get 5
                    i32.const 6
                    i32.shl
                    i32.or
                    local.set 4
                    i32.const 2
                    local.set 7
                    br 2 (;@6;)
                  end
                  local.get 5
                  i32.const -16
                  i32.and
                  i32.const -32
                  i32.eq
                  if  ;; label = @8
                    i32.const 65533
                    local.set 4
                    local.get 8
                    i32.const 3
                    i32.lt_u
                    br_if 1 (;@7;)
                    i32.const 1
                    local.set 7
                    local.get 6
                    i32.const 1
                    i32.add
                    i32.load8_u
                    local.tee 8
                    i32.const 192
                    i32.and
                    i32.const 128
                    i32.ne
                    br_if 2 (;@6;)
                    local.get 6
                    i32.const 2
                    i32.add
                    i32.load8_u
                    local.tee 6
                    i32.const 192
                    i32.and
                    i32.const 128
                    i32.ne
                    br_if 2 (;@6;)
                    local.get 8
                    i32.const 63
                    i32.and
                    i32.const 6
                    i32.shl
                    local.get 5
                    i32.const 15
                    i32.and
                    local.tee 8
                    i32.const 12
                    i32.shl
                    i32.or
                    local.tee 5
                    i32.const 2048
                    i32.lt_u
                    local.get 8
                    i32.const 13
                    i32.le_u
                    local.get 5
                    i32.const 55295
                    i32.gt_u
                    i32.and
                    i32.or
                    br_if 2 (;@6;)
                    local.get 5
                    local.get 6
                    i32.const 63
                    i32.and
                    i32.or
                    local.set 4
                    i32.const 3
                    local.set 7
                    br 2 (;@6;)
                  end
                  i32.const 1
                  local.set 7
                  i32.const 65533
                  local.set 4
                  local.get 5
                  i32.const 248
                  i32.and
                  i32.const 240
                  i32.ne
                  local.get 8
                  i32.const 4
                  i32.lt_u
                  i32.or
                  br_if 1 (;@6;)
                  local.get 6
                  i32.const 1
                  i32.add
                  i32.load8_u
                  local.tee 8
                  i32.const 192
                  i32.and
                  i32.const 128
                  i32.ne
                  br_if 1 (;@6;)
                  local.get 6
                  i32.const 2
                  i32.add
                  i32.load8_u
                  local.tee 12
                  i32.const 192
                  i32.and
                  i32.const 128
                  i32.ne
                  br_if 1 (;@6;)
                  local.get 6
                  i32.const 3
                  i32.add
                  i32.load8_u
                  local.tee 6
                  i32.const 192
                  i32.and
                  i32.const 128
                  i32.ne
                  br_if 1 (;@6;)
                  local.get 8
                  i32.const 63
                  i32.and
                  i32.const 12
                  i32.shl
                  local.get 5
                  i32.const 7
                  i32.and
                  i32.const 18
                  i32.shl
                  i32.or
                  local.tee 5
                  i32.const 65536
                  i32.sub
                  i32.const 1048575
                  i32.gt_u
                  br_if 1 (;@6;)
                  local.get 6
                  i32.const 63
                  i32.and
                  local.get 12
                  i32.const 63
                  i32.and
                  i32.const 6
                  i32.shl
                  i32.or
                  local.get 5
                  i32.or
                  local.set 4
                  i32.const 4
                  local.set 7
                  br 1 (;@6;)
                end
                i32.const 1
                local.set 7
              end
              local.get 7
              local.get 11
              i32.add
              local.set 11
              i32.const 1
            end
            if  ;; label = @5
              block  ;; label = @6
                block  ;; label = @7
                  block  ;; label = @8
                    block  ;; label = @9
                      block  ;; label = @10
                        local.get 4
                        i32.const 9
                        i32.sub
                        br_table 3 (;@7;) 1 (;@9;) 4 (;@6;) 4 (;@6;) 2 (;@8;) 0 (;@10;)
                      end
                      local.get 4
                      i32.const 92
                      i32.ne
                      if  ;; label = @10
                        local.get 4
                        i32.const 34
                        i32.ne
                        br_if 4 (;@6;)
                        local.get 3
                        i32.const 16
                        i32.add
                        local.get 10
                        local.get 9
                        i32.const 65932
                        i32.const 2
                        call $runtime.stringConcat
                        local.get 3
                        i32.load offset=20
                        local.set 9
                        local.get 3
                        i32.load offset=16
                        br 8 (;@2;)
                      end
                      local.get 3
                      i32.const 24
                      i32.add
                      local.get 10
                      local.get 9
                      i32.const 65934
                      i32.const 2
                      call $runtime.stringConcat
                      local.get 3
                      i32.load offset=28
                      local.set 9
                      local.get 3
                      i32.load offset=24
                      br 7 (;@2;)
                    end
                    local.get 3
                    i32.const 32
                    i32.add
                    local.get 10
                    local.get 9
                    i32.const 65936
                    i32.const 2
                    call $runtime.stringConcat
                    local.get 3
                    i32.load offset=36
                    local.set 9
                    local.get 3
                    i32.load offset=32
                    br 6 (;@2;)
                  end
                  local.get 3
                  i32.const 40
                  i32.add
                  local.get 10
                  local.get 9
                  i32.const 65938
                  i32.const 2
                  call $runtime.stringConcat
                  local.get 3
                  i32.load offset=44
                  local.set 9
                  local.get 3
                  i32.load offset=40
                  br 5 (;@2;)
                end
                local.get 3
                i32.const 48
                i32.add
                local.get 10
                local.get 9
                i32.const 65940
                i32.const 2
                call $runtime.stringConcat
                local.get 3
                i32.load offset=52
                local.set 9
                local.get 3
                i32.load offset=48
                br 4 (;@2;)
              end
              local.get 3
              i32.const 4
              i32.const 3
              call $runtime.alloc
              local.tee 5
              i32.store offset=84
              local.get 3
              local.get 5
              i32.store offset=88
              local.get 3
              local.get 5
              i32.store offset=80
              local.get 3
              local.get 5
              i32.store offset=76
              local.get 3
              local.get 5
              i32.store offset=72
              local.get 4
              i32.const 127
              i32.le_s
              if  ;; label = @6
                i32.const 1
                local.set 7
                i32.const 0
                local.set 6
                br 2 (;@4;)
              end
              local.get 4
              i32.const 2047
              i32.le_u
              if  ;; label = @6
                local.get 4
                i32.const 63
                i32.and
                i32.const -128
                i32.or
                local.set 6
                local.get 4
                i32.const 6
                i32.shr_u
                i32.const -64
                i32.or
                local.set 4
                i32.const 2
                local.set 7
                br 2 (;@4;)
              end
              i32.const 3
              local.set 7
              i32.const 0
              local.set 12
              local.get 4
              i32.const 2147481600
              i32.and
              i32.const 55296
              i32.eq
              if  ;; label = @6
                i32.const 189
                local.set 8
                i32.const 191
                local.set 6
                i32.const 239
                local.set 4
                br 3 (;@3;)
              end
              local.get 4
              i32.const 65535
              i32.le_u
              if  ;; label = @6
                local.get 4
                i32.const 63
                i32.and
                i32.const -128
                i32.or
                local.set 8
                local.get 4
                i32.const 6
                i32.shr_u
                i32.const 63
                i32.and
                i32.const -128
                i32.or
                local.set 6
                local.get 4
                i32.const 12
                i32.shr_u
                i32.const -32
                i32.or
                local.set 4
                br 3 (;@3;)
              end
              local.get 4
              i32.const 63
              i32.and
              i32.const -128
              i32.or
              local.set 12
              local.get 4
              i32.const 6
              i32.shr_u
              i32.const 63
              i32.and
              i32.const -128
              i32.or
              local.set 8
              local.get 4
              i32.const 12
              i32.shr_u
              i32.const 63
              i32.and
              i32.const -128
              i32.or
              local.set 6
              i32.const 4
              local.set 7
              local.get 4
              i32.const 18
              i32.shr_u
              i32.const -16
              i32.or
              local.set 4
              br 2 (;@3;)
            end
            i32.const 67404
            local.get 13
            i32.store
            local.get 0
            local.get 9
            i32.store offset=4
            local.get 0
            local.get 10
            i32.store
            local.get 3
            i32.const 96
            i32.add
            global.set $__stack_pointer
            return
          end
          i32.const 0
          local.set 8
          i32.const 0
          local.set 12
        end
        local.get 5
        local.get 4
        i32.store8
        local.get 5
        local.get 6
        i32.store8 offset=1
        local.get 5
        local.get 8
        i32.store8 offset=2
        local.get 5
        local.get 12
        i32.store8 offset=3
        local.get 3
        i32.const 8
        i32.add
        local.get 10
        local.get 9
        local.get 5
        local.get 7
        call $runtime.stringConcat
        local.get 3
        i32.load offset=12
        local.set 9
        local.get 3
        i32.load offset=8
      end
      local.tee 10
      i32.store offset=68
      br 0 (;@1;)
    end
    unreachable)
  (func $runtime.hashmapNext (type 0) (param i32 i32 i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const 80
    i32.sub
    local.tee 6
    global.set $__stack_pointer
    local.get 6
    i32.const 18
    i32.store offset=4
    local.get 6
    i32.const 12
    i32.add
    i32.const 0
    i32.const 68
    memory.fill
    local.get 6
    i32.const 67404
    i32.load
    local.tee 8
    i32.store
    i32.const 67404
    local.get 6
    i32.store
    block  ;; label = @1
      local.get 1
      i32.eqz
      br_if 0 (;@1;)
      local.get 6
      local.get 1
      i32.load
      local.tee 5
      i32.store offset=8
      local.get 5
      i32.eqz
      if  ;; label = @2
        local.get 1
        local.get 0
        i32.load
        local.tee 5
        i32.store
        local.get 1
        i32.const 1
        local.get 0
        i32.load8_u offset=20
        local.tee 4
        i32.shl
        i32.const 0
        local.get 4
        i32.const 31
        i32.le_u
        select
        i32.store offset=4
        local.get 6
        local.get 5
        i32.store offset=12
        local.get 1
        call $runtime.fastrand
        local.get 1
        i32.load offset=4
        i32.const 1
        i32.sub
        i32.and
        i32.store offset=12
        local.get 1
        call $runtime.fastrand
        i32.const 7
        i32.and
        local.tee 5
        i32.store8 offset=21
        local.get 1
        local.get 1
        i32.load offset=12
        local.tee 4
        i32.store offset=8
        local.get 0
        i32.load offset=12
        local.set 7
        local.get 0
        i32.load offset=16
        local.set 9
        local.get 1
        local.get 5
        i32.store8 offset=20
        local.get 1
        local.get 1
        i32.load
        local.tee 5
        local.get 4
        local.get 7
        local.get 9
        i32.add
        i32.const 3
        i32.shl
        i32.const 12
        i32.add
        i32.mul
        i32.add
        local.tee 4
        i32.store offset=16
        local.get 6
        local.get 5
        i32.store offset=16
        local.get 6
        local.get 4
        i32.store offset=20
      end
      loop  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 1
            i32.load8_u offset=22
            i32.eqz
            if  ;; label = @5
              local.get 1
              i32.load8_u offset=20
              local.set 5
              br 1 (;@4;)
            end
            local.get 1
            i32.load8_u offset=20
            local.set 5
            local.get 1
            i32.load offset=8
            local.get 1
            i32.load offset=12
            i32.ne
            br_if 0 (;@4;)
            local.get 5
            local.get 1
            i32.load8_u offset=21
            i32.ne
            br_if 0 (;@4;)
            i32.const 0
            local.set 5
            br 1 (;@3;)
          end
          local.get 6
          local.get 1
          i32.load offset=16
          local.tee 4
          i32.store offset=24
          local.get 5
          i32.const 8
          i32.ge_u
          if  ;; label = @4
            local.get 1
            i32.const 0
            i32.store8 offset=20
            local.get 4
            i32.eqz
            br_if 3 (;@1;)
            local.get 1
            local.get 4
            i32.load offset=8
            local.tee 4
            i32.store offset=16
            local.get 6
            local.get 4
            i32.store offset=28
            i32.const 0
            local.set 5
          end
          local.get 6
          local.get 4
          i32.store offset=32
          local.get 4
          i32.eqz
          if  ;; label = @4
            local.get 1
            local.get 1
            i32.load offset=8
            i32.const 1
            i32.add
            local.tee 4
            i32.store offset=8
            local.get 1
            i32.load offset=4
            local.get 4
            i32.le_u
            if  ;; label = @5
              local.get 1
              i32.const 1
              i32.store8 offset=22
              local.get 1
              i32.const 0
              i32.store offset=8
              i32.const 0
              local.set 4
            end
            local.get 1
            local.get 1
            i32.load
            local.tee 5
            local.get 0
            i32.load offset=16
            local.get 0
            i32.load offset=12
            i32.add
            i32.const 3
            i32.shl
            i32.const 12
            i32.add
            local.get 4
            i32.mul
            i32.add
            local.tee 4
            i32.store offset=16
            local.get 6
            local.get 5
            i32.store offset=36
            local.get 6
            local.get 4
            i32.store offset=40
            br 2 (;@2;)
          end
          local.get 6
          local.get 4
          i32.store offset=44
          local.get 4
          local.get 5
          i32.add
          i32.load8_u
          i32.eqz
          if  ;; label = @4
            local.get 1
            local.get 5
            i32.const 1
            i32.add
            i32.store8 offset=20
            br 2 (;@2;)
          end
          local.get 0
          i32.load offset=12
          local.set 7
          local.get 6
          local.get 4
          i32.store offset=48
          local.get 6
          local.get 4
          local.get 5
          local.get 7
          i32.mul
          i32.add
          i32.const 12
          i32.add
          local.tee 5
          i32.store offset=52
          local.get 2
          local.get 5
          local.get 7
          memory.copy
          local.get 6
          local.get 1
          i32.load
          local.tee 5
          i32.store offset=56
          local.get 6
          local.get 0
          i32.load
          local.tee 4
          i32.store offset=60
          local.get 4
          local.get 5
          i32.eq
          if  ;; label = @4
            local.get 6
            local.get 1
            i32.load offset=16
            local.tee 2
            i32.store offset=64
            local.get 6
            local.get 2
            local.get 0
            i32.load offset=12
            i32.const 3
            i32.shl
            i32.add
            local.get 0
            i32.load offset=16
            local.tee 0
            local.get 1
            i32.load8_u offset=20
            i32.mul
            i32.add
            i32.const 12
            i32.add
            local.tee 2
            i32.store offset=68
            local.get 3
            local.get 2
            local.get 0
            memory.copy
            i32.const 1
            local.set 5
            local.get 1
            local.get 1
            i32.load8_u offset=20
            i32.const 1
            i32.add
            i32.store8 offset=20
            br 1 (;@3;)
          end
          local.get 1
          local.get 1
          i32.load8_u offset=20
          i32.const 1
          i32.add
          i32.store8 offset=20
          local.get 6
          local.get 0
          i32.load offset=32
          local.tee 7
          i32.store offset=72
          local.get 6
          local.get 0
          i32.load offset=36
          local.tee 4
          i32.store offset=76
          local.get 4
          i32.eqz
          br_if 2 (;@1;)
          i32.const 1
          local.set 5
          local.get 0
          local.get 2
          local.get 3
          local.get 2
          local.get 0
          i32.load offset=12
          local.get 0
          i32.load offset=4
          local.get 7
          local.get 4
          call_indirect (type 0)
          call $runtime.hashmapGet
          i32.const 1
          i32.and
          i32.eqz
          br_if 1 (;@2;)
        end
      end
      i32.const 67404
      local.get 8
      i32.store
      local.get 6
      i32.const 80
      i32.add
      global.set $__stack_pointer
      local.get 5
      return
    end
    call $runtime.nilPanic
    unreachable)
  (func $github.com/weisyn/v1/contracts/sdk/go/framework.QueryBalance (type 15) (param i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32)
    (local i32 i32 i32)
    global.get $__stack_pointer
    i32.const 32
    i32.sub
    local.tee 21
    global.set $__stack_pointer
    local.get 21
    i32.const 2
    i32.store offset=20
    i32.const 67404
    i32.load
    local.set 22
    i32.const 67404
    local.get 21
    i32.const 16
    i32.add
    i32.store
    local.get 21
    local.get 22
    i32.store offset=16
    i32.const 20
    i32.const 3
    call $runtime.alloc
    local.tee 20
    local.get 19
    i32.store8 offset=19
    local.get 20
    local.get 18
    i32.store8 offset=18
    local.get 20
    local.get 17
    i32.store8 offset=17
    local.get 20
    local.get 16
    i32.store8 offset=16
    local.get 20
    local.get 15
    i32.store8 offset=15
    local.get 20
    local.get 14
    i32.store8 offset=14
    local.get 20
    local.get 13
    i32.store8 offset=13
    local.get 20
    local.get 12
    i32.store8 offset=12
    local.get 20
    local.get 11
    i32.store8 offset=11
    local.get 20
    local.get 10
    i32.store8 offset=10
    local.get 20
    local.get 9
    i32.store8 offset=9
    local.get 20
    local.get 8
    i32.store8 offset=8
    local.get 20
    local.get 7
    i32.store8 offset=7
    local.get 20
    local.get 6
    i32.store8 offset=6
    local.get 20
    local.get 5
    i32.store8 offset=5
    local.get 20
    local.get 4
    i32.store8 offset=4
    local.get 20
    local.get 3
    i32.store8 offset=3
    local.get 20
    local.get 2
    i32.store8 offset=2
    local.get 20
    local.get 1
    i32.store8 offset=1
    local.get 20
    local.get 0
    i32.store8
    local.get 21
    local.get 20
    i32.store offset=24
    local.get 21
    local.get 20
    i32.store offset=28
    local.get 21
    i32.const 8
    i32.add
    local.get 20
    i32.const 20
    call $github.com/weisyn/v1/contracts/sdk/go/framework.AllocateBytes
    i32.const 67404
    local.get 22
    i32.store
    local.get 21
    i32.const 32
    i32.add
    global.set $__stack_pointer)
  (func $runtime.memequal (type 0) (param i32 i32 i32 i32) (result i32)
    (local i32 i32)
    i32.const 0
    local.set 3
    block (result i32)  ;; label = @1
      loop  ;; label = @2
        local.get 2
        local.get 2
        local.get 3
        i32.eq
        br_if 1 (;@1;)
        drop
        local.get 1
        local.get 3
        i32.add
        local.set 4
        local.get 0
        local.get 3
        i32.add
        local.get 3
        i32.const 1
        i32.add
        local.set 3
        i32.load8_u
        local.get 4
        i32.load8_u
        i32.eq
        br_if 0 (;@2;)
      end
      local.get 3
      i32.const 1
      i32.sub
    end
    local.get 2
    i32.ge_u)
  (func $runtime.hash32 (type 0) (param i32 i32 i32 i32) (result i32)
    (local i32 i32)
    local.get 0
    i32.eqz
    local.get 1
    i32.const 0
    i32.ne
    i32.and
    local.get 1
    i32.const 0
    i32.lt_s
    i32.or
    i32.eqz
    if  ;; label = @1
      local.get 1
      i32.const 2147483644
      i32.and
      local.set 5
      local.get 1
      i32.const -962287725
      i32.mul
      local.get 2
      i32.xor
      i32.const -1130422988
      i32.xor
      local.set 4
      local.get 0
      local.set 3
      local.get 1
      local.set 2
      loop  ;; label = @2
        local.get 2
        i32.const 4
        i32.ge_u
        if  ;; label = @3
          local.get 3
          i32.load align=1
          local.get 4
          i32.add
          i32.const -962287725
          i32.mul
          local.tee 4
          i32.const 16
          i32.shr_u
          local.get 4
          i32.xor
          local.set 4
          local.get 2
          i32.const 4
          i32.sub
          local.set 2
          local.get 3
          i32.const 4
          i32.add
          local.set 3
          br 1 (;@2;)
        end
      end
      local.get 0
      local.get 5
      i32.add
      local.set 0
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            block  ;; label = @5
              local.get 1
              i32.const 3
              i32.and
              i32.const 1
              i32.sub
              br_table 2 (;@3;) 1 (;@4;) 0 (;@5;) 3 (;@2;)
            end
            local.get 0
            i32.load8_u offset=2
            i32.const 16
            i32.shl
            local.get 4
            i32.add
            local.set 4
          end
          local.get 0
          i32.load8_u offset=1
          i32.const 8
          i32.shl
          local.get 4
          i32.add
          local.set 4
        end
        local.get 4
        local.get 0
        i32.load8_u
        i32.add
        i32.const -962287725
        i32.mul
        local.tee 0
        i32.const 24
        i32.shr_u
        local.get 0
        i32.xor
        local.set 4
      end
      local.get 4
      return
    end
    i32.const 66139
    i32.const 37
    call $runtime.runtimePanicAt
    unreachable)
  (func $runtime.runtimePanicAt (type 5) (param i32 i32)
    i32.const 66028
    i32.const 22
    call $runtime.printstring
    local.get 0
    local.get 1
    call $runtime.printstring
    i32.const 10
    call $runtime.putchar
    unreachable)
  (func $runtime.printstring (type 5) (param i32 i32)
    local.get 1
    i32.const 0
    local.get 1
    i32.const 0
    i32.gt_s
    select
    local.set 1
    loop  ;; label = @1
      local.get 1
      if  ;; label = @2
        local.get 0
        i32.load8_u
        call $runtime.putchar
        local.get 1
        i32.const 1
        i32.sub
        local.set 1
        local.get 0
        i32.const 1
        i32.add
        local.set 0
        br 1 (;@1;)
      end
    end)
  (func $runtime.putchar (type 1) (param i32)
    (local i32 i32)
    i32.const 67224
    i32.load
    local.tee 1
    i32.const 119
    i32.le_u
    if  ;; label = @1
      i32.const 67224
      local.get 1
      i32.const 1
      i32.add
      local.tee 2
      i32.store
      local.get 1
      i32.const 67228
      i32.add
      local.get 0
      i32.store8
      local.get 0
      i32.const 255
      i32.and
      i32.const 10
      i32.ne
      local.get 1
      i32.const 119
      i32.ne
      i32.and
      i32.eqz
      if  ;; label = @2
        i32.const 66616
        local.get 2
        i32.store
        i32.const 1
        i32.const 66612
        i32.const 1
        i32.const 67408
        call $runtime.fd_write
        drop
        i32.const 67224
        i32.const 0
        i32.store
      end
      return
    end
    call $runtime.lookupPanic
    unreachable)
  (func $malloc (type 4) (param i32) (result i32)
    (local i32 i32 i32)
    global.get $__stack_pointer
    i32.const 32
    i32.sub
    local.tee 1
    global.set $__stack_pointer
    local.get 1
    i32.const 2
    i32.store offset=20
    i32.const 67404
    i32.load
    local.set 3
    i32.const 67404
    local.get 1
    i32.const 16
    i32.add
    i32.store
    local.get 1
    local.get 3
    i32.store offset=16
    block  ;; label = @1
      local.get 0
      if  ;; label = @2
        local.get 0
        i32.const 0
        i32.lt_s
        br_if 1 (;@1;)
        local.get 1
        local.get 0
        i32.const 3
        call $runtime.alloc
        local.tee 2
        i32.store offset=24
        local.get 1
        local.get 2
        i32.store offset=28
        local.get 1
        local.get 0
        i32.store offset=8
        local.get 1
        local.get 0
        i32.store offset=4
        local.get 1
        local.get 2
        i32.store
        local.get 1
        local.get 2
        i32.store offset=12
        local.get 1
        i32.const 12
        i32.add
        local.get 1
        call $runtime.hashmapBinarySet
      end
      i32.const 67404
      local.get 3
      i32.store
      local.get 1
      i32.const 32
      i32.add
      global.set $__stack_pointer
      local.get 2
      return
    end
    call $runtime.slicePanic
    unreachable)
  (func $runtime.hashmapBinarySet (type 5) (param i32 i32)
    i32.const 66620
    local.get 0
    local.get 1
    local.get 0
    i32.const 66632
    i32.load
    i32.const 66624
    i32.load
    local.get 0
    call $runtime.hash32
    call $runtime.hashmapSet)
  (func $free (type 1) (param i32)
    (local i32)
    global.get $__stack_pointer
    i32.const 16
    i32.sub
    local.tee 1
    global.set $__stack_pointer
    block  ;; label = @1
      local.get 0
      if  ;; label = @2
        local.get 1
        local.get 0
        i32.store offset=12
        local.get 1
        i32.const 12
        i32.add
        local.get 1
        call $runtime.hashmapBinaryGet
        i32.const 1
        i32.and
        i32.eqz
        br_if 1 (;@1;)
        local.get 1
        local.get 0
        i32.store
        local.get 1
        call $runtime.hashmapBinaryDelete
      end
      local.get 1
      i32.const 16
      i32.add
      global.set $__stack_pointer
      return
    end
    i32.const 65968
    call $runtime._panic
    unreachable)
  (func $runtime.hashmapBinaryGet (type 2) (param i32 i32) (result i32)
    i32.const 66620
    local.get 0
    local.get 1
    local.get 0
    i32.const 66632
    i32.load
    i32.const 66624
    i32.load
    local.get 0
    call $runtime.hash32
    call $runtime.hashmapGet)
  (func $runtime.hashmapBinaryDelete (type 1) (param i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const 32
    i32.sub
    local.tee 1
    global.set $__stack_pointer
    local.get 1
    i32.const 24
    i32.add
    i64.const 0
    i64.store
    local.get 1
    i64.const 0
    i64.store offset=16
    local.get 1
    i32.const 6
    i32.store offset=4
    i32.const 67404
    i32.load
    local.set 4
    i32.const 67404
    local.get 1
    i32.store
    local.get 1
    local.get 4
    i32.store
    i32.const 1
    local.get 0
    i32.const 66632
    i32.load
    i32.const 66624
    i32.load
    i32.const 0
    call $runtime.hash32
    local.tee 2
    i32.const 24
    i32.shr_u
    local.get 2
    i32.const 16777216
    i32.lt_u
    select
    local.set 7
    i32.const 66620
    local.get 2
    call $runtime.hashmapBucketAddrForHash
    local.set 2
    block  ;; label = @1
      loop  ;; label = @2
        local.get 1
        local.get 2
        i32.store offset=8
        local.get 1
        local.get 2
        i32.store offset=12
        local.get 2
        i32.eqz
        br_if 1 (;@1;)
        local.get 2
        i32.const 12
        i32.add
        local.set 8
        i32.const 0
        local.set 3
        block  ;; label = @3
          loop  ;; label = @4
            local.get 3
            i32.const 8
            i32.ne
            if  ;; label = @5
              local.get 1
              local.get 8
              i32.const 66632
              i32.load
              local.tee 9
              local.get 3
              i32.mul
              i32.add
              local.tee 5
              i32.store offset=16
              block  ;; label = @6
                local.get 2
                local.get 3
                i32.add
                local.tee 10
                i32.load8_u
                local.get 7
                i32.ne
                br_if 0 (;@6;)
                local.get 1
                i32.const 66644
                i32.load
                local.tee 11
                i32.store offset=20
                local.get 1
                i32.const 66648
                i32.load
                local.tee 6
                i32.store offset=24
                local.get 6
                i32.eqz
                br_if 3 (;@3;)
                local.get 0
                local.get 5
                local.get 9
                local.get 11
                local.get 6
                call_indirect (type 0)
                i32.const 1
                i32.and
                i32.eqz
                br_if 0 (;@6;)
                local.get 10
                i32.const 0
                i32.store8
                local.get 5
                i32.const 0
                i32.const 66632
                i32.load
                memory.fill
                local.get 2
                i32.const 66632
                i32.load
                i32.const 3
                i32.shl
                i32.add
                i32.const 66636
                i32.load
                local.tee 0
                local.get 3
                i32.mul
                i32.add
                i32.const 12
                i32.add
                i32.const 0
                local.get 0
                memory.fill
                i32.const 66628
                i32.const 66628
                i32.load
                i32.const 1
                i32.sub
                i32.store
                br 5 (;@1;)
              end
              local.get 3
              i32.const 1
              i32.add
              local.set 3
              br 1 (;@4;)
            end
          end
          local.get 2
          i32.load offset=8
          local.set 2
          br 1 (;@2;)
        end
      end
      call $runtime.nilPanic
      unreachable
    end
    i32.const 67404
    local.get 4
    i32.store
    local.get 1
    i32.const 32
    i32.add
    global.set $__stack_pointer)
  (func $runtime._panic (type 1) (param i32)
    i32.const 66021
    i32.const 7
    call $runtime.printstring
    local.get 0
    i32.load
    local.get 0
    i32.load offset=4
    call $runtime.printstring
    i32.const 10
    call $runtime.putchar
    unreachable)
  (func $calloc (type 2) (param i32 i32) (result i32)
    (local i32 i32)
    global.get $__stack_pointer
    i32.const 16
    i32.sub
    local.tee 2
    global.set $__stack_pointer
    i32.const 67404
    i32.load
    local.set 3
    i32.const 67404
    local.get 2
    i32.store
    local.get 0
    local.get 1
    i32.mul
    call $malloc
    i32.const 67404
    local.get 3
    i32.store
    local.get 2
    i32.const 16
    i32.add
    global.set $__stack_pointer)
  (func $realloc (type 2) (param i32 i32) (result i32)
    (local i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const 32
    i32.sub
    local.tee 2
    global.set $__stack_pointer
    local.get 2
    i32.const 2
    i32.store offset=20
    i32.const 67404
    i32.load
    local.set 4
    i32.const 67404
    local.get 2
    i32.const 16
    i32.add
    i32.store
    local.get 2
    local.get 4
    i32.store offset=16
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          local.get 1
          i32.eqz
          if  ;; label = @4
            local.get 0
            call $free
            br 1 (;@3;)
          end
          local.get 1
          i32.const 0
          i32.lt_s
          br_if 1 (;@2;)
          local.get 2
          local.get 1
          i32.const 3
          call $runtime.alloc
          local.tee 3
          i32.store offset=24
          local.get 2
          local.get 3
          i32.store offset=28
          local.get 0
          if  ;; label = @4
            local.get 2
            local.get 0
            i32.store offset=12
            local.get 2
            i32.const 12
            i32.add
            local.get 2
            call $runtime.hashmapBinaryGet
            i32.const 1
            i32.and
            i32.eqz
            br_if 3 (;@1;)
            local.get 3
            local.get 2
            i32.load
            local.get 2
            i32.load offset=4
            local.tee 5
            local.get 1
            local.get 1
            local.get 5
            i32.gt_u
            select
            memory.copy
            local.get 2
            local.get 0
            i32.store
            local.get 2
            call $runtime.hashmapBinaryDelete
          end
          local.get 2
          local.get 1
          i32.store offset=8
          local.get 2
          local.get 1
          i32.store offset=4
          local.get 2
          local.get 3
          i32.store
          local.get 2
          local.get 3
          i32.store offset=12
          local.get 2
          i32.const 12
          i32.add
          local.get 2
          call $runtime.hashmapBinarySet
        end
        i32.const 67404
        local.get 4
        i32.store
        local.get 2
        i32.const 32
        i32.add
        global.set $__stack_pointer
        local.get 3
        return
      end
      call $runtime.slicePanic
      unreachable
    end
    i32.const 66000
    call $runtime._panic
    unreachable)
  (func $_start (type 7)
    (local i32 i32)
    i32.const 67220
    memory.size
    i32.const 16
    i32.shl
    local.tee 0
    i32.store
    call $runtime.calculateHeapAddresses
    i32.const 67348
    i32.load
    local.tee 1
    i32.const 0
    local.get 0
    local.get 1
    i32.sub
    memory.fill
    call $arc4random
    local.set 0
    call $arc4random
    drop
    i32.const 66608
    local.get 0
    i32.const 1
    i32.or
    i32.store
    i32.const 67220
    memory.size
    i32.const 16
    i32.shl
    i32.store
    i32.const 0
    call $runtime.proc_exit)
  (func $runtime.calculateHeapAddresses (type 7)
    (local i32)
    i32.const 67348
    i32.const 67220
    i32.load
    local.tee 0
    local.get 0
    i32.const 67632
    i32.sub
    i32.const 65
    i32.div_u
    i32.sub
    local.tee 0
    i32.store
    i32.const 67356
    local.get 0
    i32.const 67696
    i32.sub
    i32.const 4
    i32.shr_u
    i32.store)
  (func $runtime.markRoots (type 5) (param i32 i32)
    (local i32)
    loop  ;; label = @1
      local.get 0
      local.get 1
      i32.ge_u
      i32.eqz
      if  ;; label = @2
        block  ;; label = @3
          local.get 0
          i32.load
          local.tee 2
          i32.const 67696
          i32.lt_u
          br_if 0 (;@3;)
          local.get 2
          i32.const 67348
          i32.load
          i32.ge_u
          br_if 0 (;@3;)
          local.get 2
          i32.const 67696
          i32.sub
          i32.const 4
          i32.shr_u
          local.tee 2
          call $_runtime.gcBlock_.state
          i32.const 255
          i32.and
          i32.eqz
          br_if 0 (;@3;)
          local.get 2
          call $_runtime.gcBlock_.findHead
          local.tee 2
          call $_runtime.gcBlock_.state
          i32.const 255
          i32.and
          i32.const 3
          i32.eq
          br_if 0 (;@3;)
          local.get 2
          call $runtime.startMark
        end
        local.get 0
        i32.const 4
        i32.add
        local.set 0
        br 1 (;@1;)
      end
    end)
  (func $_runtime.gcBlock_.state (type 4) (param i32) (result i32)
    i32.const 67348
    i32.load
    local.get 0
    i32.const 2
    i32.shr_u
    i32.add
    i32.load8_u
    local.get 0
    i32.const 1
    i32.shl
    i32.const 6
    i32.and
    i32.shr_u
    i32.const 3
    i32.and)
  (func $_runtime.gcBlock_.markFree (type 1) (param i32)
    (local i32)
    i32.const 67348
    i32.load
    local.get 0
    i32.const 2
    i32.shr_u
    i32.add
    local.tee 1
    local.get 1
    i32.load8_u
    i32.const 3
    local.get 0
    i32.const 1
    i32.shl
    i32.const 6
    i32.and
    i32.shl
    i32.const -1
    i32.xor
    i32.and
    i32.store8)
  (func $runtime.growHeap (type 3) (result i32)
    (local i32 i32 i32)
    memory.size
    memory.grow
    i32.const -1
    i32.ne
    local.tee 1
    if  ;; label = @1
      memory.size
      local.set 0
      i32.const 67220
      i32.load
      local.set 2
      i32.const 67220
      local.get 0
      i32.const 16
      i32.shl
      i32.store
      i32.const 67348
      i32.load
      local.set 0
      call $runtime.calculateHeapAddresses
      i32.const 67348
      i32.load
      local.get 0
      local.get 2
      local.get 0
      i32.sub
      memory.copy
    end
    local.get 1)
  (func $runtime.startMark (type 1) (param i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const 128
    i32.sub
    local.tee 5
    global.set $__stack_pointer
    local.get 5
    i32.const 4
    i32.add
    i32.const 0
    i32.const 124
    memory.fill
    local.get 5
    local.get 0
    i32.store
    local.get 0
    i32.const 3
    call $_runtime.gcBlock_.setState
    i32.const 1
    local.set 3
    block  ;; label = @1
      loop  ;; label = @2
        local.get 3
        i32.const 0
        i32.gt_s
        if  ;; label = @3
          local.get 3
          i32.const 1
          i32.sub
          local.tee 3
          i32.const 31
          i32.gt_u
          br_if 2 (;@1;)
          block  ;; label = @4
            local.get 5
            local.get 3
            i32.const 2
            i32.shl
            i32.add
            i32.load
            local.tee 2
            i32.const 4
            i32.shl
            local.tee 1
            i32.const 67696
            i32.add
            local.tee 10
            i32.load
            local.tee 0
            i32.eqz
            if  ;; label = @5
              i32.const 0
              local.set 7
              i32.const 1
              local.set 9
              i32.const 1
              local.set 8
              i32.const 1
              local.set 6
              br 1 (;@4;)
            end
            block (result i32)  ;; label = @5
              local.get 0
              i32.const 1
              i32.and
              if  ;; label = @6
                local.get 0
                i32.const 6
                i32.shr_u
                local.set 6
                local.get 0
                i32.const 1
                i32.shr_u
                i32.const 31
                i32.and
                local.set 8
                i32.const 0
                br 1 (;@5;)
              end
              local.get 0
              i32.load
              local.set 8
              i32.const 0
              local.set 6
              local.get 0
              i32.const 4
              i32.add
            end
            local.tee 7
            i32.eqz
            local.set 9
            local.get 7
            br_if 0 (;@4;)
            local.get 6
            i32.eqz
            br_if 2 (;@2;)
          end
          block  ;; label = @4
            block  ;; label = @5
              local.get 2
              call $_runtime.gcBlock_.state
              i32.const 255
              i32.and
              i32.const 1
              i32.sub
              br_table 0 (;@5;) 1 (;@4;) 0 (;@5;) 1 (;@4;)
            end
            local.get 2
            i32.const 1
            i32.add
            local.set 2
          end
          local.get 2
          i32.const 4
          i32.shl
          local.tee 0
          i32.const 67696
          i32.add
          local.set 4
          local.get 0
          local.get 1
          i32.sub
          i32.const 16
          i32.sub
          local.set 1
          i32.const 67348
          i32.load
          local.set 11
          loop  ;; label = @4
            block  ;; label = @5
              local.get 1
              local.set 0
              local.get 4
              local.get 11
              i32.ge_u
              br_if 0 (;@5;)
              local.get 0
              i32.const 16
              i32.add
              local.set 1
              local.get 4
              i32.const 16
              i32.add
              local.set 4
              local.get 2
              call $_runtime.gcBlock_.state
              local.get 2
              i32.const 1
              i32.add
              local.set 2
              i32.const 255
              i32.and
              i32.const 2
              i32.eq
              br_if 1 (;@4;)
            end
          end
          local.get 10
          i32.const 16
          i32.add
          local.set 2
          i32.const 0
          local.set 4
          loop  ;; label = @4
            local.get 0
            i32.eqz
            br_if 2 (;@2;)
            block  ;; label = @5
              local.get 2
              i32.load
              local.tee 1
              i32.const 67696
              i32.lt_u
              br_if 0 (;@5;)
              local.get 1
              i32.const 67348
              i32.load
              i32.ge_u
              br_if 0 (;@5;)
              block  ;; label = @6
                local.get 9
                i32.eqz
                if  ;; label = @7
                  local.get 7
                  local.get 4
                  i32.const 3
                  i32.shr_u
                  i32.add
                  i32.load8_u
                  local.get 4
                  i32.const 7
                  i32.and
                  i32.shr_u
                  i32.const 1
                  i32.and
                  br_if 1 (;@6;)
                  br 2 (;@5;)
                end
                i32.const 1
                local.get 4
                i32.shl
                local.get 6
                i32.and
                i32.eqz
                local.get 4
                i32.const 31
                i32.gt_u
                i32.or
                br_if 1 (;@5;)
              end
              local.get 1
              i32.const 67696
              i32.sub
              i32.const 4
              i32.shr_u
              local.tee 1
              call $_runtime.gcBlock_.state
              i32.const 255
              i32.and
              i32.eqz
              br_if 0 (;@5;)
              local.get 1
              call $_runtime.gcBlock_.findHead
              local.tee 1
              call $_runtime.gcBlock_.state
              i32.const 255
              i32.and
              i32.const 3
              i32.eq
              br_if 0 (;@5;)
              local.get 1
              i32.const 3
              call $_runtime.gcBlock_.setState
              local.get 3
              i32.const 32
              i32.eq
              if  ;; label = @6
                i32.const 67401
                i32.const 1
                i32.store8
                i32.const 32
                local.set 3
                br 1 (;@5;)
              end
              local.get 3
              i32.const 31
              i32.gt_u
              br_if 4 (;@1;)
              local.get 5
              local.get 3
              i32.const 2
              i32.shl
              i32.add
              local.get 1
              i32.store
              local.get 3
              i32.const 1
              i32.add
              local.set 3
            end
            local.get 4
            i32.const 1
            i32.add
            local.tee 1
            i32.const 0
            local.get 1
            local.get 8
            i32.ne
            select
            local.set 4
            local.get 0
            i32.const 4
            i32.sub
            local.set 0
            local.get 2
            i32.const 4
            i32.add
            local.set 2
            br 0 (;@4;)
          end
          unreachable
        end
      end
      local.get 5
      i32.const 128
      i32.add
      global.set $__stack_pointer
      return
    end
    call $runtime.lookupPanic
    unreachable)
  (func $_runtime.gcBlock_.setState (type 5) (param i32 i32)
    (local i32)
    i32.const 67348
    i32.load
    local.get 0
    i32.const 2
    i32.shr_u
    i32.add
    local.tee 2
    local.get 2
    i32.load8_u
    local.get 1
    local.get 0
    i32.const 1
    i32.shl
    i32.const 6
    i32.and
    i32.shl
    i32.or
    i32.store8)
  (func $runtime.hashmapGet (type 0) (param i32 i32 i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const 48
    i32.sub
    local.tee 4
    global.set $__stack_pointer
    local.get 4
    i32.const 32
    i32.add
    i64.const 0
    i64.store
    local.get 4
    i32.const 40
    i32.add
    i32.const 0
    i32.store
    local.get 4
    i64.const 0
    i64.store offset=24
    local.get 4
    i32.const 7
    i32.store offset=12
    i32.const 67404
    i32.load
    local.set 6
    i32.const 67404
    local.get 4
    i32.const 8
    i32.add
    i32.store
    local.get 4
    local.get 6
    i32.store offset=8
    local.get 4
    local.get 0
    local.get 3
    call $runtime.hashmapBucketAddrForHash
    local.tee 5
    i32.store offset=16
    i32.const 1
    local.get 3
    i32.const 24
    i32.shr_u
    local.get 3
    i32.const 16777216
    i32.lt_u
    select
    local.set 10
    block  ;; label = @1
      block  ;; label = @2
        loop  ;; label = @3
          block  ;; label = @4
            local.get 4
            local.get 5
            i32.store offset=20
            local.get 5
            i32.eqz
            br_if 0 (;@4;)
            local.get 5
            i32.const 12
            i32.add
            local.set 7
            i32.const 0
            local.set 3
            loop  ;; label = @5
              local.get 3
              i32.const 8
              i32.ne
              if  ;; label = @6
                local.get 4
                local.get 7
                local.get 0
                i32.load offset=12
                local.tee 8
                local.get 3
                i32.mul
                i32.add
                local.tee 11
                i32.store offset=24
                local.get 4
                local.get 7
                local.get 8
                i32.const 3
                i32.shl
                i32.add
                local.get 0
                i32.load offset=16
                local.get 3
                i32.mul
                i32.add
                local.tee 12
                i32.store offset=28
                block  ;; label = @7
                  local.get 3
                  local.get 5
                  i32.add
                  i32.load8_u
                  local.get 10
                  i32.ne
                  br_if 0 (;@7;)
                  local.get 4
                  local.get 0
                  i32.load offset=24
                  local.tee 13
                  i32.store offset=32
                  local.get 4
                  local.get 0
                  i32.load offset=28
                  local.tee 9
                  i32.store offset=36
                  local.get 9
                  i32.eqz
                  br_if 6 (;@1;)
                  local.get 1
                  local.get 11
                  local.get 8
                  local.get 13
                  local.get 9
                  call_indirect (type 0)
                  i32.const 1
                  i32.and
                  i32.eqz
                  br_if 0 (;@7;)
                  local.get 2
                  local.get 12
                  local.get 0
                  i32.load offset=16
                  memory.copy
                  br 5 (;@2;)
                end
                local.get 3
                i32.const 1
                i32.add
                local.set 3
                br 1 (;@5;)
              end
            end
            local.get 4
            local.get 5
            i32.load offset=8
            local.tee 5
            i32.store offset=40
            br 1 (;@3;)
          end
        end
        local.get 2
        i32.const 0
        local.get 0
        i32.load offset=16
        memory.fill
      end
      i32.const 67404
      local.get 6
      i32.store
      local.get 4
      i32.const 48
      i32.add
      global.set $__stack_pointer
      local.get 5
      i32.const 0
      i32.ne
      return
    end
    call $runtime.nilPanic
    unreachable)
  (func $runtime.hashmapBucketAddrForHash (type 2) (param i32 i32) (result i32)
    local.get 0
    if  ;; label = @1
      local.get 0
      i32.load
      local.get 0
      i32.load offset=16
      local.get 0
      i32.load offset=12
      i32.add
      i32.const 3
      i32.shl
      i32.const 12
      i32.add
      i32.const -1
      i32.const -1
      local.get 0
      i32.load8_u offset=20
      local.tee 0
      i32.shl
      i32.const -1
      i32.xor
      local.get 0
      i32.const 31
      i32.gt_u
      select
      local.get 1
      i32.and
      i32.mul
      i32.add
      return
    end
    call $runtime.nilPanic
    unreachable)
  (func $runtime.hashmapSet (type 8) (param i32 i32 i32 i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i64 i64)
    global.get $__stack_pointer
    i32.const 224
    i32.sub
    local.tee 4
    global.set $__stack_pointer
    local.get 4
    i32.const 45
    i32.store offset=36
    local.get 4
    i32.const 40
    i32.add
    i32.const 0
    i32.const 180
    memory.fill
    local.get 4
    i32.const 67404
    i32.load
    local.tee 16
    i32.store offset=32
    i32.const 67404
    local.get 4
    i32.const 32
    i32.add
    i32.store
    block  ;; label = @1
      block  ;; label = @2
        local.get 0
        i32.eqz
        br_if 0 (;@2;)
        block  ;; label = @3
          local.get 0
          i32.load8_u offset=20
          local.tee 5
          i32.const 29
          i32.gt_u
          br_if 0 (;@3;)
          local.get 0
          i32.load offset=8
          i32.const 6
          local.get 5
          i32.shl
          i32.le_u
          br_if 0 (;@3;)
          i32.const 40
          i32.const 61525
          call $runtime.alloc
          local.tee 3
          local.get 0
          i32.load
          local.tee 6
          i32.store
          local.get 3
          local.get 0
          i64.load offset=4 align=4
          i64.store offset=4 align=4
          local.get 3
          local.get 0
          i64.load offset=12 align=4
          i64.store offset=12 align=4
          local.get 3
          local.get 0
          i32.load8_u offset=20
          i32.store8 offset=20
          local.get 3
          local.get 0
          i32.load offset=24
          local.tee 7
          i32.store offset=24
          local.get 3
          local.get 0
          i32.load offset=28
          local.tee 8
          i32.store offset=28
          local.get 3
          local.get 0
          i32.load offset=32
          local.tee 9
          i32.store offset=32
          local.get 3
          local.get 0
          i32.load offset=36
          local.tee 10
          i32.store offset=36
          local.get 4
          local.get 3
          i32.store offset=40
          local.get 4
          local.get 10
          i32.store offset=60
          local.get 4
          local.get 9
          i32.store offset=56
          local.get 4
          local.get 8
          i32.store offset=52
          local.get 4
          local.get 7
          i32.store offset=48
          local.get 4
          local.get 6
          i32.store offset=44
          local.get 3
          i32.const 0
          i32.store offset=8
          call $runtime.fastrand
          local.set 6
          local.get 3
          local.get 5
          i32.const 1
          i32.add
          local.tee 5
          i32.store8 offset=20
          local.get 3
          local.get 6
          i32.store offset=4
          local.get 3
          local.get 0
          i32.load offset=16
          local.get 0
          i32.load offset=12
          i32.add
          i32.const 3
          i32.shl
          i32.const 12
          i32.add
          local.get 5
          i32.shl
          i32.const 0
          call $runtime.alloc
          local.tee 5
          i32.store
          local.get 4
          i32.const 16
          i32.add
          i64.const 0
          i64.store
          local.get 4
          i32.const 24
          i32.add
          i64.const 0
          i64.store
          local.get 4
          local.get 5
          i32.store offset=64
          local.get 4
          i64.const 0
          i64.store offset=8
          local.get 4
          local.get 0
          i32.load offset=12
          i32.const 0
          call $runtime.alloc
          local.tee 5
          i32.store offset=68
          local.get 4
          local.get 0
          i32.load offset=16
          i32.const 0
          call $runtime.alloc
          local.tee 6
          i32.store offset=72
          loop  ;; label = @4
            local.get 0
            local.get 4
            i32.const 8
            i32.add
            local.get 5
            local.get 6
            call $runtime.hashmapNext
            i32.const 1
            i32.and
            if  ;; label = @5
              local.get 4
              local.get 3
              i32.load offset=32
              local.tee 8
              i32.store offset=76
              local.get 4
              local.get 3
              i32.load offset=36
              local.tee 7
              i32.store offset=80
              local.get 7
              i32.eqz
              br_if 3 (;@2;)
              local.get 3
              local.get 5
              local.get 6
              local.get 5
              local.get 3
              i32.load offset=12
              local.get 3
              i32.load offset=4
              local.get 8
              local.get 7
              call_indirect (type 0)
              call $runtime.hashmapSet
              br 1 (;@4;)
            end
          end
          local.get 4
          local.get 3
          i32.load
          local.tee 7
          i32.store offset=104
          local.get 4
          local.get 3
          i32.load offset=24
          local.tee 8
          i32.store offset=108
          local.get 4
          local.get 7
          i32.store offset=84
          local.get 4
          local.get 3
          i32.load offset=28
          local.tee 9
          i32.store offset=112
          local.get 4
          local.get 8
          i32.store offset=88
          local.get 4
          local.get 3
          i32.load offset=32
          local.tee 6
          i32.store offset=116
          local.get 4
          local.get 9
          i32.store offset=92
          local.get 4
          local.get 3
          i32.load offset=36
          local.tee 5
          i32.store offset=120
          local.get 4
          local.get 6
          i32.store offset=96
          local.get 3
          i64.load offset=4 align=4
          local.set 17
          local.get 3
          i64.load offset=12 align=4
          local.set 18
          local.get 3
          i32.load8_u offset=20
          local.set 3
          local.get 4
          local.get 5
          i32.store offset=100
          local.get 0
          local.get 5
          i32.store offset=36
          local.get 0
          local.get 9
          i32.store offset=28
          local.get 0
          local.get 8
          i32.store offset=24
          local.get 0
          local.get 3
          i32.store8 offset=20
          local.get 0
          local.get 18
          i64.store offset=12 align=4
          local.get 0
          local.get 17
          i64.store offset=4 align=4
          local.get 0
          local.get 7
          i32.store
          local.get 0
          local.get 6
          i32.store offset=32
          local.get 4
          local.get 6
          i32.store offset=124
          local.get 4
          local.get 5
          i32.store offset=128
          local.get 5
          i32.eqz
          br_if 1 (;@2;)
          local.get 1
          local.get 0
          i32.load offset=12
          local.get 0
          i32.load offset=4
          local.get 6
          local.get 5
          call_indirect (type 0)
          local.set 3
        end
        local.get 4
        local.get 0
        local.get 3
        call $runtime.hashmapBucketAddrForHash
        local.tee 6
        i32.store offset=132
        i32.const 1
        local.get 3
        i32.const 24
        i32.shr_u
        local.get 3
        i32.const 16777216
        i32.lt_u
        select
        local.set 10
        i32.const 0
        local.set 3
        i32.const 0
        local.set 7
        i32.const 0
        local.set 9
        i32.const 0
        local.set 8
        loop  ;; label = @3
          block  ;; label = @4
            local.get 4
            local.get 3
            i32.store offset=148
            local.get 4
            local.get 6
            local.tee 5
            i32.store offset=152
            local.get 4
            local.get 7
            i32.store offset=144
            local.get 4
            local.get 9
            i32.store offset=140
            local.get 4
            local.get 8
            i32.store offset=136
            local.get 5
            i32.eqz
            br_if 0 (;@4;)
            local.get 5
            i32.const 12
            i32.add
            local.set 6
            i32.const 0
            local.set 3
            loop  ;; label = @5
              block  ;; label = @6
                local.get 4
                local.get 9
                i32.store offset=160
                local.get 4
                local.get 7
                i32.store offset=164
                local.get 4
                local.get 8
                i32.store offset=156
                local.get 3
                i32.const 8
                i32.eq
                br_if 0 (;@6;)
                local.get 4
                local.get 6
                local.get 0
                i32.load offset=12
                local.tee 13
                local.get 3
                i32.mul
                i32.add
                local.tee 14
                i32.store offset=168
                local.get 4
                local.get 6
                local.get 13
                i32.const 3
                i32.shl
                i32.add
                local.get 0
                i32.load offset=16
                local.get 3
                i32.mul
                i32.add
                local.tee 15
                i32.store offset=172
                local.get 4
                local.get 7
                local.get 14
                local.get 3
                local.get 5
                i32.add
                local.tee 11
                i32.load8_u
                local.get 7
                i32.or
                local.tee 12
                select
                local.tee 7
                i32.store offset=184
                local.get 4
                local.get 8
                local.get 11
                local.get 12
                select
                local.tee 8
                i32.store offset=176
                local.get 4
                local.get 9
                local.get 15
                local.get 12
                select
                local.tee 9
                i32.store offset=180
                block  ;; label = @7
                  local.get 11
                  i32.load8_u
                  local.get 10
                  i32.ne
                  br_if 0 (;@7;)
                  local.get 4
                  local.get 0
                  i32.load offset=24
                  local.tee 12
                  i32.store offset=188
                  local.get 4
                  local.get 0
                  i32.load offset=28
                  local.tee 11
                  i32.store offset=192
                  local.get 11
                  i32.eqz
                  br_if 5 (;@2;)
                  local.get 1
                  local.get 14
                  local.get 13
                  local.get 12
                  local.get 11
                  call_indirect (type 0)
                  i32.const 1
                  i32.and
                  i32.eqz
                  br_if 0 (;@7;)
                  local.get 15
                  local.get 2
                  local.get 0
                  i32.load offset=16
                  memory.copy
                  br 6 (;@1;)
                end
                local.get 3
                i32.const 1
                i32.add
                local.set 3
                br 1 (;@5;)
              end
            end
            local.get 4
            local.get 5
            i32.load offset=8
            local.tee 6
            i32.store offset=196
            local.get 5
            local.set 3
            br 1 (;@3;)
          end
        end
        local.get 7
        i32.eqz
        if  ;; label = @3
          local.get 0
          i32.load offset=16
          local.get 0
          i32.load offset=12
          i32.add
          i32.const 3
          i32.shl
          i32.const 12
          i32.add
          i32.const 0
          call $runtime.alloc
          local.set 5
          local.get 0
          local.get 0
          i32.load offset=8
          i32.const 1
          i32.add
          i32.store offset=8
          local.get 4
          local.get 5
          i32.store offset=204
          local.get 4
          local.get 5
          i32.store offset=216
          local.get 4
          local.get 5
          i32.store offset=200
          local.get 4
          local.get 5
          i32.const 12
          i32.add
          local.tee 6
          i32.store offset=208
          local.get 4
          local.get 6
          local.get 0
          i32.load offset=12
          local.tee 7
          i32.const 3
          i32.shl
          i32.add
          local.tee 8
          i32.store offset=212
          local.get 6
          local.get 1
          local.get 7
          memory.copy
          local.get 8
          local.get 2
          local.get 0
          i32.load offset=16
          memory.copy
          local.get 5
          local.get 10
          i32.store8
          local.get 3
          i32.eqz
          br_if 1 (;@2;)
          local.get 3
          local.get 5
          i32.store offset=8
          br 2 (;@1;)
        end
        local.get 0
        local.get 0
        i32.load offset=8
        i32.const 1
        i32.add
        i32.store offset=8
        local.get 7
        local.get 1
        local.get 0
        i32.load offset=12
        memory.copy
        local.get 9
        local.get 2
        local.get 0
        i32.load offset=16
        memory.copy
        local.get 8
        i32.eqz
        br_if 0 (;@2;)
        local.get 8
        local.get 10
        i32.store8
        br 1 (;@1;)
      end
      call $runtime.nilPanic
      unreachable
    end
    i32.const 67404
    local.get 16
    i32.store
    local.get 4
    i32.const 224
    i32.add
    global.set $__stack_pointer)
  (func $runtime.fastrand (type 3) (result i32)
    (local i32)
    i32.const 66608
    i32.const 66608
    i32.load
    local.tee 0
    i32.const 7
    i32.shl
    local.get 0
    i32.xor
    local.tee 0
    i32.const 1
    i32.shr_u
    local.get 0
    i32.xor
    local.tee 0
    i32.const 9
    i32.shl
    local.get 0
    i32.xor
    local.tee 0
    i32.store
    local.get 0)
  (func $_runtime.gcBlock_.findHead (type 4) (param i32) (result i32)
    (local i32 i32 i32)
    i32.const 67348
    i32.load
    local.set 2
    loop  ;; label = @1
      block  ;; label = @2
        local.get 2
        local.get 0
        i32.const 2
        i32.shr_u
        i32.add
        i32.load8_u
        local.tee 3
        i32.const 170
        i32.eq
        if  ;; label = @3
          local.get 0
          i32.const -1
          i32.xor
          i32.const -4
          i32.or
          local.set 1
          br 1 (;@2;)
        end
        i32.const -1
        local.set 1
        local.get 3
        local.get 0
        i32.const 1
        i32.shl
        i32.const 6
        i32.and
        i32.shr_u
        i32.const 3
        i32.and
        i32.const 2
        i32.eq
        br_if 0 (;@2;)
        local.get 0
        return
      end
      local.get 0
      local.get 1
      i32.add
      local.set 0
      br 0 (;@1;)
    end
    unreachable)
  (func $runtime.hashmapMake (type 3) (result i32)
    (local i32 i32 i32 i32 i32)
    global.get $__stack_pointer
    i32.const 16
    i32.sub
    local.tee 0
    global.set $__stack_pointer
    local.get 0
    i32.const 0
    i32.store offset=12
    local.get 0
    i32.const 2
    i32.store offset=4
    i32.const 67404
    i32.load
    local.set 2
    i32.const 67404
    local.get 0
    i32.store
    local.get 0
    local.get 2
    i32.store
    local.get 0
    i32.const 140
    i32.const 0
    call $runtime.alloc
    local.tee 3
    i32.store offset=8
    local.get 0
    i32.const 40
    i32.const 61525
    call $runtime.alloc
    local.tee 1
    i32.store offset=12
    call $runtime.fastrand
    local.set 4
    local.get 1
    i32.const 1
    i32.store offset=36
    local.get 1
    i32.const 2
    i32.store offset=28
    local.get 1
    i32.const 0
    i32.store8 offset=20
    local.get 1
    i64.const 34359738376
    i64.store offset=12 align=4
    local.get 1
    local.get 4
    i32.store offset=4
    local.get 1
    local.get 3
    i32.store
    i32.const 67404
    local.get 2
    i32.store
    local.get 0
    i32.const 16
    i32.add
    global.set $__stack_pointer
    local.get 1)
  (func $runtime.hashmapStringPtrHash (type 0) (param i32 i32 i32 i32) (result i32)
    local.get 0
    i32.load
    local.get 0
    i32.load offset=4
    local.get 2
    local.get 0
    call $runtime.hash32)
  (func $runtime.hashmapStringEqual (type 0) (param i32 i32 i32 i32) (result i32)
    local.get 0
    i32.load
    local.get 0
    i32.load offset=4
    local.get 1
    i32.load
    local.get 1
    i32.load offset=4
    call $runtime.stringEqual)
  (func $runtime.hashmapStringSet (type 8) (param i32 i32 i32 i32)
    (local i32 i32 i32)
    global.get $__stack_pointer
    i32.const 16
    i32.sub
    local.tee 4
    global.set $__stack_pointer
    local.get 4
    i32.const 2
    i32.store offset=4
    i32.const 67404
    i32.load
    local.set 6
    i32.const 67404
    local.get 4
    i32.store
    local.get 4
    local.get 6
    i32.store
    i32.const 8
    i32.const 69
    call $runtime.alloc
    local.tee 5
    local.get 2
    i32.store offset=4
    local.get 5
    local.get 1
    i32.store
    local.get 4
    local.get 5
    i32.store offset=8
    local.get 4
    local.get 5
    i32.store offset=12
    local.get 0
    i32.eqz
    if  ;; label = @1
      i32.const 66073
      i32.const 30
      call $runtime.runtimePanicAt
      unreachable
    end
    local.get 0
    local.get 5
    local.get 3
    local.get 1
    local.get 2
    local.get 0
    i32.load offset=4
    local.get 4
    call $runtime.hash32
    call $runtime.hashmapSet
    i32.const 67404
    local.get 6
    i32.store
    local.get 4
    i32.const 16
    i32.add
    global.set $__stack_pointer)
  (func $Hello (type 3) (result i32)
    (local i32 i32)
    global.get $__stack_pointer
    i32.const 32
    i32.sub
    local.tee 0
    global.set $__stack_pointer
    i32.const 67404
    i32.load
    local.set 1
    i32.const 67404
    local.get 0
    i32.const 16
    i32.add
    i32.store
    local.get 0
    i32.const 8
    i32.add
    i32.const 66176
    i32.const 11
    call $github.com/weisyn/v1/contracts/sdk/go/framework.SetReturnString
    i32.const 67404
    local.get 1
    i32.store
    local.get 0
    i32.load offset=8
    local.set 1
    local.get 0
    i32.const 32
    i32.add
    global.set $__stack_pointer
    i32.const 6
    i32.const 0
    local.get 1
    select)
  (func $ChainStatus (type 3) (result i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i64 i64)
    global.get $__stack_pointer
    i32.const 112
    i32.sub
    local.tee 0
    global.set $__stack_pointer
    local.get 0
    i32.const 13
    i32.store offset=52
    local.get 0
    i32.const 60
    i32.add
    i32.const 0
    i32.const 48
    memory.fill
    local.get 0
    i32.const 67404
    i32.load
    local.tee 6
    i32.store offset=48
    i32.const 67404
    local.get 0
    i32.const 48
    i32.add
    i32.store
    call $github.com/weisyn/v1/contracts/sdk/go/framework.getBlockHeight
    local.set 25
    call $github.com/weisyn/v1/contracts/sdk/go/framework.getTimestamp
    local.set 26
    local.get 0
    i32.const 20
    i32.add
    call $github.com/weisyn/v1/contracts/sdk/go/framework.GetCaller
    local.get 0
    i32.load8_u offset=20
    local.tee 3
    local.get 0
    i32.load8_u offset=21
    local.tee 4
    local.get 0
    i32.load8_u offset=22
    local.tee 7
    local.get 0
    i32.load8_u offset=23
    local.tee 8
    local.get 0
    i32.load8_u offset=24
    local.tee 9
    local.get 0
    i32.load8_u offset=25
    local.tee 10
    local.get 0
    i32.load8_u offset=26
    local.tee 11
    local.get 0
    i32.load8_u offset=27
    local.tee 12
    local.get 0
    i32.load8_u offset=28
    local.tee 13
    local.get 0
    i32.load8_u offset=29
    local.tee 14
    local.get 0
    i32.load8_u offset=30
    local.tee 15
    local.get 0
    i32.load8_u offset=31
    local.tee 16
    local.get 0
    i32.load8_u offset=32
    local.tee 17
    local.get 0
    i32.load8_u offset=33
    local.tee 18
    local.get 0
    i32.load8_u offset=34
    local.tee 19
    local.get 0
    i32.load8_u offset=35
    local.tee 20
    local.get 0
    i32.load8_u offset=36
    local.tee 21
    local.get 0
    i32.load8_u offset=37
    local.tee 22
    local.get 0
    i32.load8_u offset=38
    local.tee 23
    local.get 0
    i32.load8_u offset=39
    local.tee 24
    call $github.com/weisyn/v1/contracts/sdk/go/framework.QueryBalance
    local.get 0
    call $runtime.hashmapMake
    local.tee 2
    i32.store offset=56
    local.get 0
    local.get 2
    i32.store offset=96
    i32.const 8
    i32.const 0
    call $runtime.alloc
    local.tee 1
    local.get 25
    i64.store
    local.get 0
    local.get 1
    i32.store offset=60
    local.get 0
    local.get 1
    i32.store offset=64
    local.get 0
    local.get 1
    i32.store offset=44
    local.get 0
    i32.const 66188
    i32.store offset=40
    local.get 2
    i32.const 66547
    i32.const 12
    local.get 0
    i32.const 40
    i32.add
    local.tee 5
    call $runtime.hashmapStringSet
    i32.const 8
    i32.const 0
    call $runtime.alloc
    local.tee 1
    local.get 26
    i64.store
    local.get 0
    local.get 1
    i32.store offset=68
    local.get 0
    local.get 1
    i32.store offset=72
    local.get 0
    local.get 1
    i32.store offset=44
    local.get 0
    i32.const 66188
    i32.store offset=40
    local.get 2
    i32.const 66204
    i32.const 9
    local.get 5
    call $runtime.hashmapStringSet
    local.get 0
    i32.const 8
    i32.add
    local.get 3
    local.get 4
    local.get 7
    local.get 8
    local.get 9
    local.get 10
    local.get 11
    local.get 12
    local.get 13
    local.get 14
    local.get 15
    local.get 16
    local.get 17
    local.get 18
    local.get 19
    local.get 20
    local.get 21
    local.get 22
    local.get 23
    local.get 24
    call $_github.com/weisyn/v1/contracts/sdk/go/framework.Address_.ToString
    local.get 0
    local.get 0
    i32.load offset=8
    local.tee 3
    i32.store offset=76
    local.get 0
    i32.load offset=12
    local.set 4
    i32.const 8
    i32.const 0
    call $runtime.alloc
    local.tee 1
    local.get 4
    i32.store offset=4
    local.get 1
    local.get 3
    i32.store
    local.get 0
    local.get 1
    i32.store offset=80
    local.get 0
    local.get 1
    i32.store offset=84
    local.get 0
    local.get 1
    i32.store offset=44
    local.get 0
    i32.const 66216
    i32.store offset=40
    local.get 2
    i32.const 66232
    i32.const 6
    local.get 5
    call $runtime.hashmapStringSet
    i32.const 8
    i32.const 0
    call $runtime.alloc
    local.tee 1
    i64.const 0
    i64.store
    local.get 0
    local.get 1
    i32.store offset=88
    local.get 0
    local.get 1
    i32.store offset=92
    local.get 0
    local.get 1
    i32.store offset=44
    local.get 0
    i32.const 66240
    i32.store offset=40
    local.get 2
    i32.const 66284
    i32.const 14
    local.get 5
    call $runtime.hashmapStringSet
    local.get 2
    call $github.com/weisyn/v1/contracts/sdk/go/framework.SetReturnJSON
    local.set 2
    i32.const 67404
    local.get 6
    i32.store
    local.get 0
    i32.const 112
    i32.add
    global.set $__stack_pointer
    i32.const 6
    i32.const 0
    local.get 2
    select)
  (func $Inspect (type 3) (result i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i64)
    global.get $__stack_pointer
    i32.const 288
    i32.sub
    local.tee 0
    global.set $__stack_pointer
    local.get 0
    i32.const 47
    i32.store offset=92
    local.get 0
    i32.const 96
    i32.add
    i32.const 0
    i32.const 188
    memory.fill
    local.get 0
    i32.const 67404
    i32.load
    local.tee 24
    i32.store offset=88
    i32.const 67404
    local.get 0
    i32.const 88
    i32.add
    i32.store
    block (result i32)  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 0
            block (result i32)  ;; label = @5
              block  ;; label = @6
                block  ;; label = @7
                  local.get 0
                  block (result i32)  ;; label = @8
                    i32.const 8192
                    call $github.com/weisyn/v1/contracts/sdk/go/framework.malloc
                    local.tee 2
                    i32.eqz
                    if  ;; label = @9
                      i32.const 67400
                      i32.const 0
                      i32.const 0
                      call $github.com/weisyn/v1/contracts/sdk/go/framework.NewContractParams
                      br 1 (;@8;)
                    end
                    local.get 2
                    i32.const 8192
                    call $github.com/weisyn/v1/contracts/sdk/go/framework.getContractInitParams
                    local.tee 1
                    i32.eqz
                    if  ;; label = @9
                      i32.const 67400
                      i32.const 0
                      i32.const 0
                      call $github.com/weisyn/v1/contracts/sdk/go/framework.NewContractParams
                      br 1 (;@8;)
                    end
                    local.get 1
                    i32.const 1048577
                    i32.ge_u
                    br_if 1 (;@7;)
                    local.get 0
                    local.get 2
                    i32.store offset=96
                    local.get 2
                    local.get 1
                    local.get 1
                    call $github.com/weisyn/v1/contracts/sdk/go/framework.NewContractParams
                  end
                  local.tee 2
                  i32.store offset=100
                  local.get 0
                  local.get 2
                  i32.store offset=104
                  local.get 0
                  i32.const 48
                  i32.add
                  local.get 2
                  i32.const 66541
                  i32.const 6
                  call $_*github.com/weisyn/v1/contracts/sdk/go/framework.ContractParams_.ParseJSON
                  local.get 0
                  local.get 0
                  i32.load offset=48
                  local.tee 3
                  i32.store offset=108
                  local.get 0
                  i32.load offset=52
                  local.tee 4
                  i32.eqz
                  if  ;; label = @8
                    local.get 0
                    call $runtime.hashmapMake
                    local.tee 2
                    i32.store offset=112
                    local.get 0
                    local.get 2
                    i32.store offset=116
                    local.get 0
                    i32.const 66376
                    i32.store offset=60
                    local.get 0
                    i32.const 66216
                    i32.store offset=56
                    local.get 2
                    i32.const 66536
                    i32.const 5
                    local.get 0
                    i32.const 56
                    i32.add
                    call $runtime.hashmapStringSet
                    br 4 (;@4;)
                  end
                  local.get 3
                  local.get 4
                  i32.const 66547
                  i32.const 12
                  call $runtime.stringEqual
                  i32.const 1
                  i32.and
                  if  ;; label = @8
                    call $github.com/weisyn/v1/contracts/sdk/go/framework.getBlockHeight
                    local.set 26
                    local.get 0
                    call $runtime.hashmapMake
                    local.tee 1
                    i32.store offset=128
                    local.get 0
                    local.get 1
                    i32.store offset=140
                    local.get 0
                    i32.const 66384
                    i32.store offset=60
                    local.get 0
                    i32.const 66216
                    i32.store offset=56
                    local.get 1
                    i32.const 66541
                    i32.const 6
                    local.get 0
                    i32.const 56
                    i32.add
                    local.tee 2
                    call $runtime.hashmapStringSet
                    i32.const 8
                    i32.const 0
                    call $runtime.alloc
                    local.tee 3
                    local.get 26
                    i64.store
                    local.get 0
                    local.get 3
                    i32.store offset=132
                    local.get 0
                    local.get 3
                    i32.store offset=136
                    local.get 0
                    local.get 3
                    i32.store offset=60
                    local.get 0
                    i32.const 66188
                    i32.store offset=56
                    local.get 1
                    i32.const 66392
                    i32.const 6
                    local.get 2
                    call $runtime.hashmapStringSet
                    br 6 (;@2;)
                  end
                  local.get 3
                  local.get 4
                  i32.const 66559
                  i32.const 7
                  call $runtime.stringEqual
                  i32.const 1
                  i32.and
                  if  ;; label = @8
                    local.get 0
                    i32.const 40
                    i32.add
                    local.get 2
                    i32.const 66440
                    i32.const 7
                    call $_*github.com/weisyn/v1/contracts/sdk/go/framework.ContractParams_.ParseJSON
                    local.get 0
                    local.get 0
                    i32.load offset=40
                    local.tee 1
                    i32.store offset=152
                    local.get 0
                    local.get 1
                    i32.const 0
                    local.get 0
                    i32.load offset=44
                    local.tee 3
                    select
                    local.tee 25
                    i32.store offset=156
                    local.get 3
                    i32.eqz
                    if  ;; label = @9
                      local.get 0
                      i32.const 20
                      i32.add
                      call $github.com/weisyn/v1/contracts/sdk/go/framework.GetCaller
                      local.get 0
                      i32.load8_u offset=39
                      local.set 4
                      local.get 0
                      i32.load8_u offset=38
                      local.set 5
                      local.get 0
                      i32.load8_u offset=37
                      local.set 6
                      local.get 0
                      i32.load8_u offset=36
                      local.set 7
                      local.get 0
                      i32.load8_u offset=35
                      local.set 8
                      local.get 0
                      i32.load8_u offset=34
                      local.set 9
                      local.get 0
                      i32.load8_u offset=33
                      local.set 10
                      local.get 0
                      i32.load8_u offset=32
                      local.set 11
                      local.get 0
                      i32.load8_u offset=31
                      local.set 12
                      local.get 0
                      i32.load8_u offset=30
                      local.set 13
                      local.get 0
                      i32.load8_u offset=29
                      local.set 14
                      local.get 0
                      i32.load8_u offset=28
                      local.set 15
                      local.get 0
                      i32.load8_u offset=27
                      local.set 16
                      local.get 0
                      i32.load8_u offset=26
                      local.set 17
                      local.get 0
                      i32.load8_u offset=25
                      local.set 18
                      local.get 0
                      i32.load8_u offset=24
                      local.set 19
                      local.get 0
                      i32.load8_u offset=23
                      local.set 20
                      local.get 0
                      i32.load8_u offset=22
                      local.set 21
                      local.get 0
                      i32.load8_u offset=21
                      local.set 22
                      local.get 0
                      i32.load8_u offset=20
                      local.set 23
                      br 6 (;@3;)
                    end
                    block  ;; label = @9
                      block  ;; label = @10
                        local.get 3
                        i32.const 1
                        i32.le_s
                        if  ;; label = @11
                          local.get 0
                          local.get 1
                          i32.store offset=196
                          br 1 (;@10;)
                        end
                        i32.const 0
                        local.set 2
                        local.get 0
                        local.get 1
                        i32.const 2
                        i32.const 0
                        local.get 1
                        i32.const 2
                        i32.const 65572
                        i32.const 2
                        call $runtime.stringEqual
                        i32.const 1
                        i32.and
                        local.tee 4
                        select
                        i32.add
                        local.tee 6
                        i32.store offset=196
                        local.get 3
                        i32.const 2
                        i32.sub
                        local.get 3
                        local.get 4
                        select
                        i32.const 64
                        i32.eq
                        br_if 1 (;@9;)
                      end
                      local.get 0
                      i32.const 1
                      i32.const 65574
                      i32.const 45
                      call $github.com/weisyn/v1/contracts/sdk/go/framework.NewContractError
                      local.tee 1
                      i32.store offset=200
                      local.get 0
                      local.get 1
                      i32.store offset=204
                      br 3 (;@6;)
                    end
                    local.get 0
                    i32.const 80
                    i32.add
                    i64.const 0
                    i64.store
                    local.get 0
                    i32.const 72
                    i32.add
                    i64.const 0
                    i64.store
                    local.get 0
                    i32.const -64
                    i32.sub
                    i64.const 0
                    i64.store
                    local.get 0
                    i64.const 0
                    i64.store offset=56
                    local.get 0
                    i32.const 56
                    i32.add
                    local.set 1
                    loop  ;; label = @9
                      local.get 2
                      i32.const 63
                      i32.le_u
                      if  ;; label = @10
                        local.get 2
                        local.get 6
                        i32.add
                        local.tee 5
                        i32.load8_u
                        call $github.com/weisyn/v1/contracts/sdk/go/framework.hexCharToNibble
                        local.tee 4
                        i32.const 255
                        i32.and
                        i32.const 255
                        i32.ne
                        local.get 5
                        i32.const 1
                        i32.add
                        i32.load8_u
                        call $github.com/weisyn/v1/contracts/sdk/go/framework.hexCharToNibble
                        local.tee 5
                        i32.const 255
                        i32.and
                        i32.const 255
                        i32.ne
                        i32.and
                        if  ;; label = @11
                          local.get 1
                          local.get 5
                          local.get 4
                          i32.const 4
                          i32.shl
                          i32.or
                          i32.store8
                          local.get 1
                          i32.const 1
                          i32.add
                          local.set 1
                          local.get 2
                          i32.const 2
                          i32.add
                          local.set 2
                          br 2 (;@9;)
                        else
                          local.get 0
                          i32.const 1
                          i32.const 65786
                          i32.const 32
                          call $github.com/weisyn/v1/contracts/sdk/go/framework.NewContractError
                          local.tee 1
                          i32.store offset=208
                          local.get 0
                          local.get 1
                          i32.store offset=212
                          br 5 (;@6;)
                        end
                        unreachable
                      end
                    end
                    local.get 0
                    i32.load8_u offset=75
                    local.set 4
                    local.get 0
                    i32.load8_u offset=74
                    local.set 5
                    local.get 0
                    i32.load8_u offset=73
                    local.set 6
                    local.get 0
                    i32.load8_u offset=72
                    local.set 7
                    local.get 0
                    i32.load8_u offset=71
                    local.set 8
                    local.get 0
                    i32.load8_u offset=70
                    local.set 9
                    local.get 0
                    i32.load8_u offset=69
                    local.set 10
                    local.get 0
                    i32.load8_u offset=68
                    local.set 11
                    local.get 0
                    i32.load8_u offset=67
                    local.set 12
                    local.get 0
                    i32.load8_u offset=66
                    local.set 13
                    local.get 0
                    i32.load8_u offset=65
                    local.set 14
                    local.get 0
                    i32.load8_u offset=64
                    local.set 15
                    local.get 0
                    i32.load8_u offset=63
                    local.set 16
                    local.get 0
                    i32.load8_u offset=62
                    local.set 17
                    local.get 0
                    i32.load8_u offset=61
                    local.set 18
                    local.get 0
                    i32.load8_u offset=60
                    local.set 19
                    local.get 0
                    i32.load8_u offset=59
                    local.set 20
                    local.get 0
                    i32.load8_u offset=58
                    local.set 21
                    local.get 0
                    i32.load8_u offset=57
                    local.set 22
                    local.get 0
                    i32.load8_u offset=56
                    local.set 23
                    i32.const 0
                    local.set 1
                    i32.const 0
                    br 3 (;@5;)
                  end
                  local.get 0
                  call $runtime.hashmapMake
                  local.tee 2
                  i32.store offset=248
                  local.get 0
                  local.get 2
                  i32.store offset=272
                  local.get 0
                  i32.const 66528
                  i32.store offset=60
                  local.get 0
                  i32.const 66216
                  i32.store offset=56
                  local.get 2
                  i32.const 66536
                  i32.const 5
                  local.get 0
                  i32.const 56
                  i32.add
                  local.tee 5
                  call $runtime.hashmapStringSet
                  i32.const 8
                  i32.const 0
                  call $runtime.alloc
                  local.tee 1
                  local.get 4
                  i32.store offset=4
                  local.get 1
                  local.get 3
                  i32.store
                  local.get 0
                  local.get 1
                  i32.store offset=252
                  local.get 0
                  local.get 1
                  i32.store offset=256
                  local.get 0
                  local.get 1
                  i32.store offset=60
                  local.get 0
                  i32.const 66216
                  i32.store offset=56
                  local.get 2
                  i32.const 66541
                  i32.const 6
                  local.get 5
                  call $runtime.hashmapStringSet
                  i32.const 16
                  i32.const 69
                  call $runtime.alloc
                  local.tee 1
                  i32.const 7
                  i32.store offset=12
                  local.get 1
                  i32.const 66559
                  i32.store offset=8
                  local.get 1
                  i32.const 12
                  i32.store offset=4
                  local.get 1
                  i32.const 66547
                  i32.store
                  local.get 0
                  local.get 1
                  i32.store offset=260
                  i32.const 12
                  i32.const 0
                  call $runtime.alloc
                  local.tee 3
                  i64.const 8589934594
                  i64.store offset=4 align=4
                  local.get 3
                  local.get 1
                  i32.store
                  local.get 0
                  local.get 3
                  i32.store offset=264
                  local.get 0
                  local.get 3
                  i32.store offset=268
                  local.get 0
                  local.get 3
                  i32.store offset=60
                  local.get 0
                  i32.const 66568
                  i32.store offset=56
                  local.get 2
                  i32.const 66588
                  i32.const 9
                  local.get 5
                  call $runtime.hashmapStringSet
                  br 3 (;@4;)
                end
                call $runtime.slicePanic
                unreachable
              end
              i32.const 0
              local.set 6
              i32.const 0
              local.set 5
              i32.const 0
              local.set 4
              i32.const 66600
            end
            local.tee 2
            i32.store offset=216
            local.get 0
            local.get 1
            i32.store offset=220
            local.get 2
            i32.eqz
            br_if 1 (;@3;)
            local.get 0
            call $runtime.hashmapMake
            local.tee 2
            i32.store offset=224
            local.get 0
            local.get 2
            i32.store offset=236
            local.get 0
            i32.const 66432
            i32.store offset=60
            local.get 0
            i32.const 66216
            i32.store offset=56
            local.get 2
            i32.const 66536
            i32.const 5
            local.get 0
            i32.const 56
            i32.add
            local.tee 4
            call $runtime.hashmapStringSet
            i32.const 8
            i32.const 0
            call $runtime.alloc
            local.tee 1
            local.get 3
            i32.store offset=4
            local.get 1
            local.get 25
            i32.store
            local.get 0
            local.get 1
            i32.store offset=228
            local.get 0
            local.get 1
            i32.store offset=232
            local.get 0
            local.get 1
            i32.store offset=60
            local.get 0
            i32.const 66216
            i32.store offset=56
            local.get 2
            i32.const 66440
            i32.const 7
            local.get 4
            call $runtime.hashmapStringSet
            local.get 0
            i32.const 66496
            i32.store offset=60
            local.get 0
            i32.const 66216
            i32.store offset=56
            local.get 2
            i32.const 66504
            i32.const 4
            local.get 4
            call $runtime.hashmapStringSet
          end
          local.get 2
          call $github.com/weisyn/v1/contracts/sdk/go/framework.SetReturnJSON
          drop
          i32.const 1
          br 2 (;@1;)
        end
        local.get 23
        local.get 22
        local.get 21
        local.get 20
        local.get 19
        local.get 18
        local.get 17
        local.get 16
        local.get 15
        local.get 14
        local.get 13
        local.get 12
        local.get 11
        local.get 10
        local.get 9
        local.get 8
        local.get 7
        local.get 6
        local.get 5
        local.get 4
        call $github.com/weisyn/v1/contracts/sdk/go/framework.QueryBalance
        local.get 0
        call $runtime.hashmapMake
        local.tee 1
        i32.store offset=160
        local.get 0
        local.get 1
        i32.store offset=184
        local.get 0
        i32.const 66400
        i32.store offset=60
        local.get 0
        i32.const 66216
        i32.store offset=56
        local.get 1
        i32.const 66541
        i32.const 6
        local.get 0
        i32.const 56
        i32.add
        local.tee 2
        call $runtime.hashmapStringSet
        local.get 0
        i32.const 8
        i32.add
        local.get 23
        local.get 22
        local.get 21
        local.get 20
        local.get 19
        local.get 18
        local.get 17
        local.get 16
        local.get 15
        local.get 14
        local.get 13
        local.get 12
        local.get 11
        local.get 10
        local.get 9
        local.get 8
        local.get 7
        local.get 6
        local.get 5
        local.get 4
        call $_github.com/weisyn/v1/contracts/sdk/go/framework.Address_.ToString
        local.get 0
        local.get 0
        i32.load offset=8
        local.tee 4
        i32.store offset=164
        local.get 0
        i32.load offset=12
        local.set 5
        i32.const 8
        i32.const 0
        call $runtime.alloc
        local.tee 3
        local.get 5
        i32.store offset=4
        local.get 3
        local.get 4
        i32.store
        local.get 0
        local.get 3
        i32.store offset=168
        local.get 0
        local.get 3
        i32.store offset=172
        local.get 0
        local.get 3
        i32.store offset=60
        local.get 0
        i32.const 66216
        i32.store offset=56
        local.get 1
        i32.const 66440
        i32.const 7
        local.get 2
        call $runtime.hashmapStringSet
        i32.const 8
        i32.const 0
        call $runtime.alloc
        local.tee 3
        i64.const 0
        i64.store
        local.get 0
        local.get 3
        i32.store offset=176
        local.get 0
        local.get 3
        i32.store offset=180
        local.get 0
        local.get 3
        i32.store offset=60
        local.get 0
        i32.const 66240
        i32.store offset=56
        local.get 1
        i32.const 66559
        i32.const 7
        local.get 2
        call $runtime.hashmapStringSet
      end
      i32.const 6
      local.get 1
      call $github.com/weisyn/v1/contracts/sdk/go/framework.SetReturnJSON
      br_if 0 (;@1;)
      drop
      i32.const 0
    end
    i32.const 67404
    local.get 24
    i32.store
    local.get 0
    i32.const 288
    i32.add
    global.set $__stack_pointer)
  (func $invoke (type 3) (result i32)
    i32.const 0)
  (func (;68;) (type 0) (param i32 i32 i32 i32) (result i32)
    (local i32 i32 i32)
    global.get $__stack_pointer
    i32.const 16
    i32.sub
    local.tee 5
    global.set $__stack_pointer
    i32.const 67404
    i32.load
    local.set 6
    i32.const 67404
    local.get 5
    i32.store
    i32.const 12
    local.get 3
    call $runtime.alloc
    local.tee 4
    local.get 2
    i32.store offset=8
    local.get 4
    local.get 1
    i32.store offset=4
    local.get 4
    local.get 0
    i32.store
    i32.const 67404
    local.get 6
    i32.store
    local.get 5
    i32.const 16
    i32.add
    global.set $__stack_pointer
    local.get 4)
  (table (;0;) 5 5 funcref)
  (memory (;0;) 2)
  (global $__stack_pointer (mut i32) (i32.const 65536))
  (export "memory" (memory 0))
  (export "malloc" (func $malloc))
  (export "free" (func $free))
  (export "calloc" (func $calloc))
  (export "realloc" (func $realloc))
  (export "_start" (func $_start))
  (export "Hello" (func $Hello))
  (export "ChainStatus" (func $ChainStatus))
  (export "Inspect" (func $Inspect))
  (export "invoke" (func $invoke))
  (elem (;0;) (i32.const 1) func $runtime.hashmapStringPtrHash $runtime.hashmapStringEqual $runtime.memequal $runtime.hash32)
  (data $.rodata (i32.const 65536) "expand 32-byte k0123456789abcdef0\22:\220xinvalid address length: expected 64 hex chars\00z\00\00\00(\04\01\00|\00\01\00\c0\00\01\00framework.ContractError\00Z\00\00\00\a0\00\01\00\c0\00\01\00\0c\00\00\00\02\00\00\00\b0\00\01\00\a8\00\01\00\a8\02\01\00\f0\00\01\00\d5\00\00\00|\00\01\00\04\00Code\00\00\ca\00\00\00\b8\00\01\00\d5\00\00\00\b0\00\01\00github.com/weisyn/v1/contracts/sdk/go/framework\00\04\04Message\00invalid hex character in addressfailed to allocate return datafailed to set return dataunsupported return type-\00\00\00\c6\00\00\00t\01\01\00\d5\00\00\00l\01\01\00null{}\22\22:}{[],][\5c\22\5c\5c\5cn\5cr\5ctfree: invalid pointer\00\00\00\00\00\96\01\01\00\15\00\00\00realloc: invalid pointer\b8\01\01\00\18\00\00\00out of memorypanic: panic: runtime error: nil pointer dereferenceassignment to entry in nil mapindex out of rangeslice out of rangeunsafe.Slice/String: len out of rangeHello, WES!\00\cb\00\00\00\94\02\01\00\d5\00\00\00\8c\02\01\00timestamp\00\00\00Q\00\00\00\b0\02\01\00\d5\00\00\00\a8\02\01\00caller\00\00\eb\00\00\00\e4\02\01\00\8c\02\01\00\c0\00\01\00framework.Amount\00\00\00\00\d5\00\00\00\c0\02\01\00caller_balance\00\00\19\00\00\00\0c\03\01\00\14\03\01\00\a8\02\01\00\d5\00\00\00\fc\02\01\00T\00\00\00\1c\03\01\00\d5\00\00\00\14\03\01\00missing required field: action\00\00\00\00\00\00$\03\01\00\1e\00\00\00\f3\03\01\00\0c\00\00\00result\00\00\ff\03\01\00\07\00\00\00invalid address format\00\00h\03\01\00\16\00\00\00addressexpected 64-char hex string, e.g., 0x1234...abcd\00\8f\03\01\000\00\00\00hintunsupported action\00\00\cc\03\01\00\12\00\00\00erroractionblock_heightbalance\00\00\16\00\00\00\14\04\01\00\a8\02\01\00\d5\00\00\00\08\04\01\00supported\00\00\00\d5\00\01\00T\00\01")
  (data $.data (i32.const 66608) "\c1\82\01\00\9c\06\01\00\00\00\00\00T\07\01\00\c1\82\01\00\00\00\00\00\04\00\00\00\0c\00\00\00\01\00\00\00\00\00\00\00\03\00\00\00\00\00\00\00\04"))
