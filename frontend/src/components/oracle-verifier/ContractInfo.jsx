import React, { useState, useEffect } from 'react';
import { parseAbi } from 'viem';
import oracleVerifierAbi from '../../abi_quotes.json';
import tradingAbi from '../../abi_trade.json';

const ContractInfo = ({ publicClient, contractAddresses }) => {
  const [verifierInfo, setVerifierInfo] = useState(null);
  const [tradingInfo, setTradingInfo] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchContractInfo = async () => {
      if (!publicClient || !contractAddresses.verifier || !contractAddresses.trading) {
        return;
      }

      setLoading(true);
      setError(null);

      try {
        // Получаем информацию о контракте Oracle Verifier
        const owner = await publicClient.readContract({
          address: contractAddresses.verifier,
          abi: oracleVerifierAbi,
          functionName: 'owner',
        });

        const oracleCount = await publicClient.readContract({
          address: contractAddresses.verifier,
          abi: oracleVerifierAbi,
          functionName: 'oracleCount',
        });

        const threshold = await publicClient.readContract({
          address: contractAddresses.verifier,
          abi: oracleVerifierAbi,
          functionName: 'threshold',
        });

        const maxQuoteAge = await publicClient.readContract({
          address: contractAddresses.verifier,
          abi: oracleVerifierAbi,
          functionName: 'maxQuoteAge',
        });

        setVerifierInfo({
          owner,
          oracleCount,
          threshold,
          maxQuoteAge,
        });

        // Получаем информацию о контракте Asset Trading
        const tradingOwner = await publicClient.readContract({
          address: contractAddresses.trading,
          abi: tradingAbi,
          functionName: 'owner',
        });

        const oracleVerifierAddress = await publicClient.readContract({
          address: contractAddresses.trading,
          abi: tradingAbi,
          functionName: 'oracleVerifier',
        });

        setTradingInfo({
          owner: tradingOwner,
          oracleVerifierAddress,
        });

      } catch (err) {
        console.error('Ошибка при загрузке информации о контрактах:', err);
        setError('Не удалось загрузить информацию о контрактах.');
      } finally {
        setLoading(false);
      }
    };

    fetchContractInfo();
  }, [publicClient, contractAddresses]);

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-semibold mb-4">Информация о Смарт-Контрактах</h2>
      
      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
          {error}
        </div>
      )}
      
      {loading && (
        <div className="py-4 text-center">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-500 mx-auto"></div>
          <p className="mt-2 text-gray-500">Загрузка информации о контрактах...</p>
        </div>
      )}
      
      {!loading && verifierInfo && tradingInfo && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* OracleVerifier */}
          <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-200">
            <h3 className="text-lg font-medium mb-3">OracleVerifier</h3>
            <p className="text-sm mb-2">
              <span className="font-medium text-gray-600">Адрес:</span>{' '}
              <a 
                href={`https://testnet.bscscan.com/address/${contractAddresses.verifier}`} 
                target="_blank" 
                rel="noopener noreferrer"
                className="text-indigo-600 hover:underline font-mono"
              >
                {contractAddresses.verifier}
              </a>
            </p>
            <p className="text-sm mb-2">
              <span className="font-medium text-gray-600">Владелец:</span>{' '}
              <a 
                href={`https://testnet.bscscan.com/address/${verifierInfo.owner}`} 
                target="_blank" 
                rel="noopener noreferrer"
                className="text-indigo-600 hover:underline font-mono"
              >
                {verifierInfo.owner}
              </a>
            </p>
            <p className="text-sm mb-2">
              <span className="font-medium text-gray-600">Количество оракулов:</span> {verifierInfo.oracleCount.toString()}
            </p>
            <p className="text-sm mb-2">
              <span className="font-medium text-gray-600">Порог подписей:</span> {verifierInfo.threshold.toString()} 
              <span className="text-gray-500 ml-1">(из {verifierInfo.oracleCount.toString()})</span>
            </p>
            <p className="text-sm mb-2">
              <span className="font-medium text-gray-600">Макс. возраст котировки:</span> {verifierInfo.maxQuoteAge.toString()} секунд 
              <span className="text-gray-500 ml-1">({(Number(verifierInfo.maxQuoteAge) / 60).toFixed(1)} минут)</span>
            </p>
            
            <div className="mt-4 bg-gray-50 p-3 rounded">
              <h4 className="text-sm font-semibold text-gray-600 mb-2">Основные функции:</h4>
              <ul className="list-disc pl-5 text-sm">
                <li>Проверка подписей от доверенных оракулов</li>
                <li>Управление списком доверенных оракулов</li>
                <li>Установка порога подписей</li>
              </ul>
            </div>
          </div>
          
          {/* AssetTradingWithQuotes */}
          <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-200">
            <h3 className="text-lg font-medium mb-3">AssetTradingWithQuotes</h3>
            <p className="text-sm mb-2">
              <span className="font-medium text-gray-600">Адрес:</span>{' '}
              <a 
                href={`https://testnet.bscscan.com/address/${contractAddresses.trading}`} 
                target="_blank" 
                rel="noopener noreferrer"
                className="text-indigo-600 hover:underline font-mono"
              >
                {contractAddresses.trading}
              </a>
            </p>
            <p className="text-sm mb-2">
              <span className="font-medium text-gray-600">Владелец:</span>{' '}
              <a 
                href={`https://testnet.bscscan.com/address/${tradingInfo.owner}`} 
                target="_blank" 
                rel="noopener noreferrer"
                className="text-indigo-600 hover:underline font-mono"
              >
                {tradingInfo.owner}
              </a>
            </p>
            <p className="text-sm mb-2">
              <span className="font-medium text-gray-600">Использует OracleVerifier:</span>{' '}
              <a 
                href={`https://testnet.bscscan.com/address/${tradingInfo.oracleVerifierAddress}`} 
                target="_blank" 
                rel="noopener noreferrer"
                className="text-indigo-600 hover:underline font-mono"
              >
                {tradingInfo.oracleVerifierAddress}
              </a>
            </p>
            
            <div className="mt-4 bg-gray-50 p-3 rounded">
              <h4 className="text-sm font-semibold text-gray-600 mb-2">Основные функции:</h4>
              <ul className="list-disc pl-5 text-sm">
                <li>Обработка подписанных сообщений с ценами</li>
                <li>Генерация событий при успешной/неуспешной проверке</li>
                <li>Демонстрация работы системы без хранения токенов</li>
              </ul>
            </div>
          </div>
          
          {/* Событийная модель */}
          <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-200 md:col-span-2">
            <h3 className="text-lg font-medium mb-3">Событийная модель</h3>
            
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-2">
              <div>
                <h4 className="text-sm font-semibold text-gray-600 mb-2">События OracleVerifier:</h4>
                <ul className="list-disc pl-5 text-sm">
                  <li><span className="font-mono text-xs">OracleAdded(address indexed oracle)</span></li>
                  <li><span className="font-mono text-xs">OracleRemoved(address indexed oracle)</span></li>
                  <li><span className="font-mono text-xs">ThresholdUpdated(uint256 newThreshold)</span></li>
                  <li><span className="font-mono text-xs">MaxQuoteAgeUpdated(uint256 newMaxAge)</span></li>
                </ul>
              </div>
              
              <div>
                <h4 className="text-sm font-semibold text-gray-600 mb-2">События AssetTradingWithQuotes:</h4>
                <ul className="list-disc pl-5 text-sm">
                  <li><span className="font-mono text-xs">PriceVerified(address indexed user, bytes32 messageHash)</span></li>
                  <li><span className="font-mono text-xs">PriceRejected(address indexed user, bytes32 messageHash)</span></li>
                </ul>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default ContractInfo; 