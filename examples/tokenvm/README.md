<h1 align='center'>
  Hyper-WASM
</h1>
<p align="center">
  Wasm Runtime Supported by HyperSdk
</p>

---

## Todo

- Hyper-Wasm-Sdk standardisation and abstraction.
- Proper offset and pointer dealings
- Write reusable go components.
- Sample Smart contracts.
- Add support for crypto libraries.
- Functional MsgValue and contract balance.
- Compute units metering.

## Rules:
 - Declare public functions as extern with C layout, e.g., pub extern "C" fn simplefunction()
 - Use `#[no_mangle]` macro for external functions
 - Ensure structs follow C layout, i.e., `#[repr(C)]`
 - Public functions should use u/i/f/32/64 for input.
 - Hyper-WASM communicates with Rust contracts using pointers due to WASM's limited support for data types (limited to u/i/f/32/64)

### memoryHex Usage in Hyper-WASM:
 - memoryHex is supported and writes to the zero pointer before transaction execution.
 - Usecases for this behavior should be explored.

### Hyper-WASM State Storage:
 - Hyper-WASM provides 32 slots for state storage, numbered 0 to 31.

### Hyper Wasm Sdk

  Hyper wasm sdk provides necessary modules required for writing a contract.
  ```js
       sdk
        |-- allocator
        |-- state
        |-- tx
        |-- utils
  ```
  Necessary declarations for contract
  ```rust
  use hyper_wasm_sdk::allocator::*;
  ```
  For state interactions
  ```rust
  use hyper_wasm_sdk::state::*;

      state
        |-- fn store_uint(slot: u32, x:u64);
        |-- fn store_int(slot: u32, x:i64);
        |-- fn store_float(slot: u32, x:f64);
        |-- fn store_string(slot:u32, ptr: u32, size: u32);
        |-- fn store_bytes(slot:u32, ptr: u32, size: u32);
        |-- fn store_append_array(slot:u32, ptr: u32, size: u32);
        |-- fn pop_array(slot:u32, position: u32, size: u32);
        |-- fn insert_array(slot:u32, ptr: u32, size: u32, position: u32);
        |-- fn replace_array(slot:u32, ptr: u32, size: u32, position: u32);
        |-- fn delete_array(slot: u32);
        |-- fn get_uint(slot: u32) -> u64;
        |-- fn get_int(slot: u32) -> i64;
        |-- fn get_float(slot: u32) -> f64;
        |-- fn get_string(slot: u32) -> u64;
        |-- fn get_bytes(slot: u32) -> u64;
        |-- fn get_array_at_index(slot: u32, size: u32, position: u32) -> u64;
        |-- fn CALL();  // ⚠️ not supported yet
        |-- fn DELEGATECALL(); // ⚠️ not supported yet
  ```
### During Output:
 - Output must always be `u64`
 - Left 32 bits represent the pointer, while the rest represent the size


####  Example for output:
  
  ```rust
      // Get pointer to MyStruct: 
             let struct_ptr: *const MyStruct = &my_struct;
      // Extract u32 value from the pointer: 
             let u32_value: u32 = struct_ptr as *const _ as u32;
      // Calculate size of the struct:   
             let size_of_struct = mem::size_of_val(&my_struct);
      // Output Packing
             std::mem::forget(msg_sender);
             return ((ptr as u64) << 32) | len as u64;
```

### Function Inputs:
 - 0 inputs: Implies `tx::info` is not passed during the function call.
 - 1 input: Implies `tx::info` is passed during the function call.
 - 2 inputs: Implies `tx::info` is passed along with an input struct during the function call.

### During input:
 - For non-supported data types, provide inputHex
 - Hyper-WASM writes it to contract memory and returns the pointer
 - Use the pointer to access the desired data type

### Example for mutable and non-mutable structs sent during input:
```rust
    // Immutable Struct:
        #[no_mangle]
        pub extern "C" fn immutable_struct_function(tx_info_ptr: *const tx::Info){
            let tx_info = unsafe { &*tx_info_ptr };
        }
    
    // Mutable Struct:
        #[no_mangle]
        pub extern "C" fn mutable_struct_function(tx_info_ptr: u32){
            // Safety: Ensure that the input is not null
            if input.is_null() {
                //take necessary action or return  any appropriate error code
            }
            // Safety: Dereference the pointer to access the struct
            let tx_info = unsafe { &mut *(tx_info_ptr as *mut tx::Info }; 
        }
```

## Demos

- Build
```bash
./scripts/run.sh;
./scripts/build.sh; #build cli
./build/token-cli key import demo.pk
./build/token-cli chain import-anr
```

- Deploy Contract
```bash
./build/token-cli action deploy-contract
```

- Transact

```bash
./build/token-cli action transact
```

#### Bonus: Watch Activity in Real-Time
To provide a better sense of what is actually happening on-chain, the
`token-cli` comes bundled with a simple explorer that logs all blocks/txs that
occur on-chain. You can run this utility by running the following command from
this location:
```bash
./build/token-cli chain watch
```

## Creating a Devnet
_In the world of Avalanche, we refer to short-lived, test-focused Subnets as devnets._

Using [avalanche-ops](https://github.com/ava-labs/avalanche-ops), we can create a private devnet (running on a
custom Primary Network with traffic scoped to the deployer IP) across any number of regions and nodes
in ~30 minutes with a single script.

### Step 1: Install Dependencies
#### Install and Configure `aws-cli`
Install the [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html). This is used to
authenticate to AWS and manipulate CloudFormation.

Once you've installed the AWS CLI, run `aws configure sso` to login to AWS locally. See
[the avalanche-ops documentation](https://github.com/ava-labs/avalanche-ops#permissions) for additional details.
Set a `profile_name` when logging in, as it will be referenced directly by avalanche-ops commands. **Do not set
an SSO Session Name (not supported).**

#### Install `yq`
Install `yq` using [Homebrew](https://brew.sh/). `yq` is used to manipulate the YAML files produced by
`avalanche-ops`.

You can install `yq` using the following command:
```bash
brew install yq
```

### Step 2: Deploy Devnet on AWS
Once all dependencies are installed, we can create our devnet using a single script. This script deploys
10 validators (equally split between us-west-2, us-east-2, and eu-west-1):
```bash
./scripts/deploy.devnet.sh
```

_When devnet creation is complete, this script will emit commands that can be used to interact
with the devnet (i.e. tx load test) and to tear it down._

#### Configuration Options
* `--arch-type`: `avalanche-ops` supports deployment to both `amd64` and `arm64` architectures
* `--anchor-nodes`/`--non-anchor-nodes`: `anchor-nodes` + `non-anchor-nodes` is the number of validators that will be on the Subnet, split equally between `--regions` (`anchor-nodes` serve as bootstrappers on the custom Primary Network, whereas `non-anchor-nodes` are just validators)
* `--regions`: `avalanche-ops` will automatically deploy validators across these regions
* `--instance-types`: `avalanche-ops` allows instance types to be configured by region (make sure it is compatible with `arch-type`)
* `--upload-artifacts-avalanchego-local-bin`: `avalanche-ops` allows a custom AvalancheGo binary to be provided for validators to run

<br>
<br>
<br>
<p align="center">
  <a href="https://github.com/ava-labs/hypersdk"><img width="40%" alt="powered-by-hypersdk" src="assets/hypersdk.png"></a>
</p>
