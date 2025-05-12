import React, { useState, useEffect } from 'react';
import { createPublicClient, createWalletClient, custom, http } from 'viem';
import { useAccount, useChains } from 'wagmi';
import { siberiumTestnet, bscTestnet } from '../WagmiProvider';
import { useRPC } from '../context/RPCContext';
import MessagesList from '../components/oracle-verifier/MessagesList';
import MessageDetail from '../components/oracle-verifier/MessageDetail';
import PriceProcessor from '../components/oracle-verifier/PriceProcessor';
import ContractInfo from '../components/oracle-verifier/ContractInfo';

const OracleVerifier = () => {
  const { apiUrl } = useRPC();
  const { address, isConnected } = useAccount();
  const [activeTab, setActiveTab] = useState('messages');
  const [publicClient, setPublicClient] = useState(null);
  const [walletClient, setWalletClient] = useState(null);
  const [selectedMessage, setSelectedMessage] = useState(null);
  
  // Адреса смарт-контрактов
  const contractAddresses = {
    verifier: "0x1B198e1A81b40D4679F97ffF1a8390A4aB072D75", // OracleVerifier
    trading: "0x91a45cD6e8Fd6B2E31b4f994fD2FFF7730aC7B7a", // AssetTradingWithQuotes
  };
  
  // Инициализация подключения к кошельку и настройка клиентов
  useEffect(() => {
    // Создаем публичный клиент для чтения из блокчейна
    const client = createPublicClient({
      chain: bscTestnet,//siberiumTestnet,
      transport: http()
    });
    setPublicClient(client);
    
    // Создаем клиент для взаимодействия с кошельком если подключен
    if (window.ethereum && isConnected) {
      const wallet = createWalletClient({
        chain: bscTestnet,//siberiumTestnet,
        transport: custom(window.ethereum)
      });
      setWalletClient(wallet);
    }
  }, [isConnected]);
  
  // Функция для подключения к MetaMask
  const connectWallet = async () => {
    if (window.ethereum) {
      try {
        // Запрос на добавление/переключение на сеть Siberium
        await window.ethereum.request({
          method: 'wallet_switchEthereumChain',
          params: [{ chainId: '0x61' }], // 111000 в hex для Siberium
        }).catch(async (switchError) => {})
        //   // Если сеть не добавлена, добавляем её
        //   if (switchError.code === 4902) {
        //     await window.ethereum.request({
        //       method: 'wallet_addEthereumChain',
        //       params: [{
        //         chainId: '0x1b198', // 111000 в hex для Siberium
        //         chainName: 'Siberium Test Network',
        //         nativeCurrency: {
        //           name: 'SIBR',
        //           symbol: 'SIBR',
        //           decimals: 18
        //         },
        //         rpcUrls: ['https://rpc.test.siberium.net'],
        //         blockExplorerUrls: ['https://explorer.test.siberium.net']
        //       }]
        //     });
        //   } else {
        //     throw switchError;
        //   }
        // });
        
        // Запрашиваем доступ к аккаунтам
        await window.ethereum.request({ method: 'eth_requestAccounts' });
        
        // После успешного подключения обновим клиенты
        const wallet = createWalletClient({
          chain: bscTestnet,//siberiumTestnet,
          transport: custom(window.ethereum)
        });
        setWalletClient(wallet);
        
      } catch (error) {
        console.error("Не удалось подключиться к кошельку:", error);
      }
    } else {
      alert("Пожалуйста, установите MetaMask!");
    }
  };

  return (
    <div className="space-y-8">
      <div className="bg-white rounded-lg shadow-md overflow-hidden">
        <div className="flex flex-wrap border-b">
          <button
            className={`px-6 py-3 text-sm font-medium ${activeTab === 'messages' ? 'bg-indigo-50 text-indigo-700 border-b-2 border-indigo-500' : 'text-gray-600 hover:text-gray-800 hover:bg-gray-50'}`}
            onClick={() => setActiveTab('messages')}
          >
            Подписи
          </button>
          <button
            className={`px-6 py-3 text-sm font-medium ${activeTab === 'verify' ? 'bg-indigo-50 text-indigo-700 border-b-2 border-indigo-500' : 'text-gray-600 hover:text-gray-800 hover:bg-gray-50'}`}
            onClick={() => setActiveTab('verify')}
          >
            Пример использования
          </button>
          <button
            className={`px-6 py-3 text-sm font-medium ${activeTab === 'contracts' ? 'bg-indigo-50 text-indigo-700 border-b-2 border-indigo-500' : 'text-gray-600 hover:text-gray-800 hover:bg-gray-50'}`}
            onClick={() => setActiveTab('contracts')}
          >
            Информация о контрактах
          </button>
        </div>
        
        <div className="p-6">
          {activeTab === 'messages' && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <MessagesList 
                apiUrl={apiUrl} 
                onSelectMessage={(message) => setSelectedMessage(message)} 
              />
              <MessageDetail message={selectedMessage} />
            </div>
          )}
          
          {activeTab === 'verify' && (
            <>
              <div className="bg-white p-5 rounded-lg shadow-md flex flex-col md:flex-row justify-between items-center mb-8">
                <div>
                  <h2 className="text-lg font-semibold mb-2">Подключение к блокчейну</h2>
                  <p className="text-sm text-gray-600">Подключенный аккаунт:</p>
                  <p className="font-mono font-medium">
                    {address ? address : 'Не подключен'}
                  </p>
                </div>
                {!isConnected && (
                  <button
                    onClick={connectWallet}
                    className="mt-4 md:mt-0 px-6 py-2 bg-indigo-600 text-white rounded-md hover:bg-indigo-700 transition-colors"
                  >
                    Подключить Кошелек
                  </button>
                )}
              </div>
              <PriceProcessor
                publicClient={publicClient}
                walletClient={walletClient}
                account={address}
                contractAddresses={contractAddresses}
                apiUrl={apiUrl}
              />
            </>
          )}
          
          
          {activeTab === 'contracts' && (
            <ContractInfo
              publicClient={publicClient}
              contractAddresses={contractAddresses}
            />
          )}
        </div>
      </div>
    </div>
  );
};

export default OracleVerifier; 