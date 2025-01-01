# Observer

## Description
Observer is a Gnosis chain shutter monitoring service built using Golang. It monitors the
shutter networks encrypted mempool and links encrypted transactions with original user
transactions(after decryption).

## How it works
1. Users submit their encrypted transactions to shutters [sequencer contract](https://github.com/shutter-network/contracts/blob/main/src/gnosh/Sequencer.sol) (encrypted mempool).
2. These transactions once submitted to the sequencer contract are indexed by the observer.
3. Observer is also connected to the p2p network of shutter and is always listening to the new decryption keys which are being gossiped by the keypers. 
4. Once the relevant decryption key for an encrypted transaction is identified, it decrypts and stores it in the DB(postgres).
5. Once all the data is available the Observer is able to identify if the users original transaction has been included according to the shutter rules and is protected from MEV.

## How to run
Clone this repository and navigate to the root directory of the project:
```shell
git clone git@github.com:shutter-network/observer.git && cd observer
```
and then execute
```shell
./observer start --rpc-url ${RPC_URL} --beacon-api-url ${BEACON_API_URL} --sequencer-contract-address ${SEQUENCER_CONTRACT_ADDRESS} --validator-registry-contract-address ${VALIDATOR_REGISTRY_CONTRACT_ADDRESS} --p2pkey ${P2P_KEY} --inclusion-delay ${INCLUSION_DELAY}
```
you can also run it inside docker.