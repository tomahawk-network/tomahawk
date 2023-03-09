KEY="mykey"
CHAINID="tomahawk_9000-1"
MONIKER="localtestnet"
KEYRING="test"
KEYALGO="eth_secp256k1"
LOGLEVEL="info"
# to trace evm
#TRACE="--trace"
TRACE=""

# validate dependencies are installed
command -v jq > /dev/null 2>&1 || { echo >&2 "jq not installed. More info: https://stedolan.github.io/jq/download/"; exit 1; }

# Reinstall daemon
rm -rf ~/.tomad*
make install

# Set client config
tomad config keyring-backend $KEYRING
tomad config chain-id $CHAINID

# if $KEY exists it should be deleted
tomad keys add $KEY --keyring-backend $KEYRING --algo $KEYALGO

# Set moniker and chain-id for tomahawk (Moniker can be anything, chain-id must be an integer)
tomad init $MONIKER --chain-id $CHAINID

# Change parameter token denominations to atoma
cat $HOME/.tomad/config/genesis.json | jq '.app_state["staking"]["params"]["bond_denom"]="atoma"' > $HOME/.tomad/config/tmp_genesis.json && mv $HOME/.tomad/config/tmp_genesis.json $HOME/.tomad/config/genesis.json
cat $HOME/.tomad/config/genesis.json | jq '.app_state["crisis"]["constant_fee"]["denom"]="atoma"' > $HOME/.tomad/config/tmp_genesis.json && mv $HOME/.tomad/config/tmp_genesis.json $HOME/.tomad/config/genesis.json
cat $HOME/.tomad/config/genesis.json | jq '.app_state["gov"]["deposit_params"]["min_deposit"][0]["denom"]="atoma"' > $HOME/.tomad/config/tmp_genesis.json && mv $HOME/.tomad/config/tmp_genesis.json $HOME/.tomad/config/genesis.json
cat $HOME/.tomad/config/genesis.json | jq '.app_state["evm"]["params"]["evm_denom"]="atoma"' > $HOME/.tomad/config/tmp_genesis.json && mv $HOME/.tomad/config/tmp_genesis.json $HOME/.tomad/config/genesis.json
cat $HOME/.tomad/config/genesis.json | jq '.app_state["inflation"]["params"]["mint_denom"]="atoma"' > $HOME/.tomad/config/tmp_genesis.json && mv $HOME/.tomad/config/tmp_genesis.json $HOME/.tomad/config/genesis.json

# Set gas limit in genesis
cat $HOME/.tomad/config/genesis.json | jq '.consensus_params["block"]["max_gas"]="10000000"' > $HOME/.tomad/config/tmp_genesis.json && mv $HOME/.tomad/config/tmp_genesis.json $HOME/.tomad/config/genesis.json

# disable produce empty block
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' 's/create_empty_blocks = true/create_empty_blocks = false/g' $HOME/.tomad/config/config.toml
  else
    sed -i 's/create_empty_blocks = true/create_empty_blocks = false/g' $HOME/.tomad/config/config.toml
fi

if [[ $1 == "pending" ]]; then
  if [[ "$OSTYPE" == "darwin"* ]]; then
      sed -i '' 's/create_empty_blocks_interval = "0s"/create_empty_blocks_interval = "30s"/g' $HOME/.tomad/config/config.toml
      sed -i '' 's/timeout_propose = "3s"/timeout_propose = "30s"/g' $HOME/.tomad/config/config.toml
      sed -i '' 's/timeout_propose_delta = "500ms"/timeout_propose_delta = "5s"/g' $HOME/.tomad/config/config.toml
      sed -i '' 's/timeout_prevote = "1s"/timeout_prevote = "10s"/g' $HOME/.tomad/config/config.toml
      sed -i '' 's/timeout_prevote_delta = "500ms"/timeout_prevote_delta = "5s"/g' $HOME/.tomad/config/config.toml
      sed -i '' 's/timeout_precommit = "1s"/timeout_precommit = "10s"/g' $HOME/.tomad/config/config.toml
      sed -i '' 's/timeout_precommit_delta = "500ms"/timeout_precommit_delta = "5s"/g' $HOME/.tomad/config/config.toml
      sed -i '' 's/timeout_commit = "5s"/timeout_commit = "150s"/g' $HOME/.tomad/config/config.toml
      sed -i '' 's/timeout_broadcast_tx_commit = "10s"/timeout_broadcast_tx_commit = "150s"/g' $HOME/.tomad/config/config.toml
  else
      sed -i 's/create_empty_blocks_interval = "0s"/create_empty_blocks_interval = "30s"/g' $HOME/.tomad/config/config.toml
      sed -i 's/timeout_propose = "3s"/timeout_propose = "30s"/g' $HOME/.tomad/config/config.toml
      sed -i 's/timeout_propose_delta = "500ms"/timeout_propose_delta = "5s"/g' $HOME/.tomad/config/config.toml
      sed -i 's/timeout_prevote = "1s"/timeout_prevote = "10s"/g' $HOME/.tomad/config/config.toml
      sed -i 's/timeout_prevote_delta = "500ms"/timeout_prevote_delta = "5s"/g' $HOME/.tomad/config/config.toml
      sed -i 's/timeout_precommit = "1s"/timeout_precommit = "10s"/g' $HOME/.tomad/config/config.toml
      sed -i 's/timeout_precommit_delta = "500ms"/timeout_precommit_delta = "5s"/g' $HOME/.tomad/config/config.toml
      sed -i 's/timeout_commit = "5s"/timeout_commit = "150s"/g' $HOME/.tomad/config/config.toml
      sed -i 's/timeout_broadcast_tx_commit = "10s"/timeout_broadcast_tx_commit = "150s"/g' $HOME/.tomad/config/config.toml
  fi
fi

# Allocate genesis accounts (cosmos formatted addresses)
tomad add-genesis-account $KEY 100000000000000000000010000atoma --keyring-backend $KEYRING

# Update total supply with claim values
validators_supply=$(cat $HOME/.tomad/config/genesis.json | jq -r '.app_state["bank"]["supply"][0]["amount"]')
# Bc is required to add this big numbers
# total_supply=$(bc <<< "$amount_to_claim+$validators_supply")
total_supply=100000000000000000000010000
cat $HOME/.tomad/config/genesis.json | jq -r --arg total_supply "$total_supply" '.app_state["bank"]["supply"][0]["amount"]=$total_supply' > $HOME/.tomad/config/tmp_genesis.json && mv $HOME/.tomad/config/tmp_genesis.json $HOME/.tomad/config/genesis.json

# Sign genesis transaction
tomad gentx $KEY 1000000000000000000000atoma --keyring-backend $KEYRING --chain-id $CHAINID

# Collect genesis tx
tomad collect-gentxs

# Run this to ensure everything worked and that the genesis file is setup correctly
tomad validate-genesis

if [[ $1 == "pending" ]]; then
  echo "pending mode is on, please wait for the first block committed."
fi

# Start the node (remove the --pruning=nothing flag if historical queries are not needed)
tomad start --pruning=nothing $TRACE --log_level $LOGLEVEL --minimum-gas-prices=0.0001atoma --json-rpc.api eth,txpool,personal,net,debug,web3 --rpc.laddr "tcp://0.0.0.0:26657" --api.enable