import React from 'react';
import { WagmiProvider as WagmiProviderLib } from 'wagmi';
import { config, siberiumTestnet, bscTestnet } from './wagmi-config';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

// Create a client for react-query
const queryClient = new QueryClient();

export function WagmiProvider({ children }) {
  return (
    <WagmiProviderLib config={config}>
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    </WagmiProviderLib>
  );
}

export { siberiumTestnet, bscTestnet }; 