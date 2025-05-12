import { createConfig, http } from 'wagmi'
import { mainnet, sepolia } from 'wagmi/chains'
import { defineChain } from 'viem'
import { injected } from 'wagmi/connectors'

export const bscTestnet = /*#__PURE__*/ defineChain({
  id: 97,
  name: 'Binance Smart Chain Testnet',
  nativeCurrency: {
    decimals: 18,
    name: 'BNB',
    symbol: 'tBNB',
  },
  rpcUrls: {
    default: { http: ['https://bsc-testnet.public.blastapi.io'] },
  },
  blockExplorers: {
    default: {
      name: 'BscScan',
      url: 'https://testnet.bscscan.com',
      apiUrl: 'https://api-testnet.bscscan.com/api',
    },
  },
  contracts: {
    multicall3: {
      address: '0xca11bde05977b3631167028862be2a173976ca11',
      blockCreated: 17422483,
    },
  },
  testnet: true,
})


// Define Siberium testnet chain
export const siberiumTestnet = defineChain({
  id: 111_000,
  name: 'Siberium Test Network',
  network: 'siberium-testnet',
  nativeCurrency: {
    name: 'SIBR',
    symbol: 'SIBR',
    decimals: 18,
  },
  rpcUrls: {
    default: { http: ['https://rpc.test.siberium.net'] },
    public: { http: ['https://rpc.test.siberium.net'] },
  },
  blockExplorers: {
    default: { name: 'Explorer', url: 'https://explorer.test.siberium.net' },
  },
})

// Create wagmi config with Siberium
// export const config = createConfig({
//   chains: [siberiumTestnet],
//   transports: {
//     [siberiumTestnet.id]: http('https://rpc.test.siberium.net'),
//   },
//   connectors: [
//     injected(),
//   ],
// }) 

export const config = createConfig({
  chains: [bscTestnet],
  transports: {
    [bscTestnet.id]: http('https://bsc-testnet.public.blastapi.io'),
  },
  connectors: [
    injected(),
  ],
}) 