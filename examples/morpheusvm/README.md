<p align="center">
  <img width="90%" alt="morpheusvm" src="assets/logo.jpeg">
</p>
<p align="center">
  The Choice is Yours
</p>

---
## What are SNARK Accounts Aka SNACS?
For background on SNARK Accounts, read discussion on [celestia forum](https://forum.celestia.org/t/celestia-snark-accounts-design-spec/1639).

Every snark proof generation requires a setup, where proving key, verfying key are generated. In anology to key based signature schemes, private key is anologous to proving key and public key is anologus to verifying key.

Address is derived from verifying key.
## How does this work?

A new auth type called SNACS is added. Users use this auth type to sign and verify their transactions.

```go
type SNACS struct {
	VKey  groth16.VerifyingKey `json:"vkey,omitempty"`
	Proof groth16.Proof        `json:"proof"`
	addr  codec.Address
}
```

The below circuit is defined in the SNACKS module. The cirucit checks if the `mimic hash of PreImage` equals `Hash` 

```go
type Circuit struct {
	PreImage frontend.Variable
	Hash     frontend.Variable `gnark:",public"`
}

func (circuit *Circuit) Define(api frontend.API) error {
	mimc, _ := mimc.NewMiMC(api)
	mimc.Write(circuit.PreImage)
	api.AssertIsEqual(circuit.Hash, mimc.Sum())
	return nil
}
```

While signing a transaction, the sha224 hash of the `msg` is mimic hashed. `msg` is the transaction digest (obtained with `tx.Digest()`) sent while signing. As this auth module uses the BN254 curve for proof generation, `msg` should be its field element, but `msg` may be larger than 254 bits, so to keep it under 254 bits, we sha224 hash msg.

```go
func (s *SNACSFactory) Sign(msg []byte) (chain.Auth, error) {
	msgHash := sha256.Sum224(msg)
	hash := MimcHash(msgHash[:])
	assignment := &Circuit{
		PreImage: frontend.Variable(msgHash[:]),
		Hash:     frontend.Variable(hash),
	}

	witness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	if err != nil {
		return nil, fmt.Errorf("error creating new witness: %s", err)
	}

	proof, err := groth16.Prove(s.CS, s.PKey, witness)
	if err != nil {
		return nil, fmt.Errorf("error generating proof: %s", err)
	}

	return &SNACS{Proof: proof, VKey: s.VKey}, nil
}
```

The Verification function logic defines the circuit-dependent or circuit-specific verification but holds the security guarantee. i.e the proof is somehow related to the msg. Our implementation of the verification function is circuit-specific, but this can be made circuit-independent if we avoid deriving public witness from `msg`(this is an extra check to verify if the proof corresponds to the public witness derived from the `msg`).

```go
func (s *SNACS) Verify(_ context.Context, msg []byte) error {
	msgHash := sha256.Sum224(msg)
	hash := MimcHash(msgHash[:])
	assignement := &Circuit{
		PreImage: frontend.Variable(msgHash[:]), // preImage can be anything, as PreImage is not public input. But filling the field is necessary for the witness to be created
		Hash:     frontend.Variable(hash),
	}
	witness, err := frontend.NewWitness(assignement, ecc.BN254.ScalarField())
	if err != nil {
		return err
	}
	pubWit, err := witness.Public()
	if err != nil {
		return err
	}
	return groth16.Verify(s.Proof, s.VKey, pubWit)
}
```
## Demo
### Launch Subnet
The first step to running this demo is to launch your own `morpheusvm` Subnet. You
can do so by running the following command from this location (may take a few
minutes):
```bash
./scripts/run.sh;
```

When the Subnet is running, you'll see the following logs emitted:
```
cluster is ready!
avalanche-network-runner is running in the background...

use the following command to terminate:

./scripts/stop.sh;
```

_By default, this allocates all funds on the network to `morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu`. The private
key for this address is `0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7`.
For convenience, this key has is also stored at `demo.pk`._

### Build `morpheus-cli`
To make it easy to interact with the `morpheusvm`, we implemented the `morpheus-cli`.
Next, you'll need to build this tool. You can use the following command:
```bash
./scripts/build.sh
```

_This command will put the compiled CLI in `./build/morpheus-cli`._

### Configure `morpheus-cli`
Next, you'll need to add the chains you created and the default key to the
`morpheus-cli`. You can use the following commands from this location to do so:
```bash
./build/morpheus-cli key import ed25519 demo.pk
```

If the key is added corretcly, you'll see the following log:
```
database: .morpheus-cli
imported address: morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu
```

Next, you'll need to store the URLs of the nodes running on your Subnet:
```bash
./build/morpheus-cli chain import-anr
```

If `morpheus-cli` is able to connect to ANR, it will emit the following logs:
```
database: .morpheus-cli
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:45778/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:58191/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:16561/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:14628/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
stored chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk uri: http://127.0.0.1:44160/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
```

_`./build/morpheus-cli chain import-anr` connects to the Avalanche Network Runner server running in
the background and pulls the URIs of all nodes tracking each chain you
created._

### Setup SNAC factory:
for using SNAC account, we need factory to be initalisedl, factory contains proving key, verifying key, constraint system.

```shell
  ./build/morpheus-cli key generate-snacs
```

_this is the snark account address, copy the address for further usage._

### Send Tokens to SNAC account
Lastly, we trigger the transfer:
```bash
./build/morpheus-cli action transfer
```
Send funds to the Snac Address copied.
The `morpheus-cli` will emit the following logs when the transfer is successful:
```
database: .morpheus-cli
address: morpheus1qqds2l0ryq5hc2ddps04384zz6rfeuvn3kyvn77hp4n5sv3ahuh6wgkt57y
chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
balance: 1000.000000000 RED
recipient: morpheus1q8rc050907hx39vfejpawjydmwe6uujw0njx9s6skzdpp3cm2he5s036p07
✔ amount: 10
continue (y/n): y
✅ txID: sceRdaoqu2AAyLdHCdQkENZaXngGjRoc8nFdGyG8D9pCbTjbk
```
### Make transactions from SNAC account:
transfer funds to morpheus1qqds2l0ryq5hc2ddps04384zz6rfeuvn3kyvn77hp4n5sv3ahuh6wgkt57y, using snac account

```shell
  ./build/morpheus-cli action transfer-snac
```

### View Balance:
```shell
  ./build/morpheus-cli key balance
```
### Bonus: Watch Activity in Real-Time
To provide a better sense of what is actually happening on-chain, the
`morpheus-cli` comes bundled with a simple explorer that logs all blocks/txs that
occur on-chain. You can run this utility by running the following command from
this location:
```bash
./build/morpheus-cli chain watch
```

If you run it correctly, you'll see the following input (will run until the
network shuts down or you exit):
```
database: .morpheus-cli
available chains: 1 excluded: []
0) chainID: 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
select chainID: 0
uri: http://127.0.0.1:45778/ext/bc/2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk
watching for new blocks on 2mQy8Q9Af9dtZvVM8pKsh2rB3cT3QNLjghpet5Mm5db4N7Hwgk 👀
height:1 txs:1 units:440 root:WspVPrHNAwBcJRJPVwt7TW6WT4E74dN8DuD3WXueQTMt5FDdi
✅ sceRdaoqu2AAyLdHCdQkENZaXngGjRoc8nFdGyG8D9pCbTjbk actor: morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu units: 440 summary (*actions.Transfer): [10.000000000 RED -> morpheus1q8rc050907hx39vfejpawjydmwe6uujw0njx9s6skzdpp3cm2he5s036p07]
```

<br>
<br>
<br>
<p align="center">
  <a href="https://github.com/ava-labs/hypersdk"><img width="40%" alt="powered-by-hypersdk" src="assets/hypersdk.png"></a>
</p>
