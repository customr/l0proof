require("@nomicfoundation/hardhat-toolbox");
require("dotenv").config();

const { RPC_URL, PRIVATE_KEY } = process.env

/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  solidity: {
    compilers: [
      {
        version: "0.8.24",
        settings: {
          optimizer: {
            enabled: true,
            runs: 200,
          },
        },
      },
    ],
  },
  paths: {
    sources: "./contracts",
    tests: "./test",
    cache: "./cache",
    artifacts: "./artifacts"
  },
  defaultNetwork: "siberium",
  networks: {
    hardhat: {},
    siberium: {
      url: RPC_URL,
      accounts: [ PRIVATE_KEY] 
    },
  },
};
