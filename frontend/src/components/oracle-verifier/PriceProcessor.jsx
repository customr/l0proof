import React, { useState, useEffect } from 'react';
import { parseAbi } from 'viem';
import tradingAbi from '../../abi_trade.json';
import oracleVerifierAbi from '../../abi_quotes.json';

const PriceProcessor = ({ publicClient, walletClient, account, contractAddresses, apiUrl }) => {
  const [selectedMessage, setSelectedMessage] = useState(null);
  const [latestMessage, setLatestMessage] = useState(null);
  const [signatures, setSignatures] = useState([]);
  const [loading, setLoading] = useState(false);
  const [txHash, setTxHash] = useState(null);
  const [txStatus, setTxStatus] = useState(null);
  const [error, setError] = useState(null);
  const [maxQuoteAge, setMaxQuoteAge] = useState(300); // Default 5 minutes
  const [messageAge, setMessageAge] = useState(0);
  const [isMessageTooOld, setIsMessageTooOld] = useState(false);

  // Load maxQuoteAge from contract
  useEffect(() => {
    const fetchMaxQuoteAge = async () => {
      if (!publicClient || !contractAddresses.verifier) {
        return;
      }
      
      try {
        const age = await publicClient.readContract({
          address: contractAddresses.verifier,
          abi: oracleVerifierAbi,
          functionName: 'maxQuoteAge',
        });
        
        setMaxQuoteAge(Number(age));
      } catch (err) {
        console.error('Ошибка при загрузке maxQuoteAge:', err);
      }
    };
    
    fetchMaxQuoteAge();
  }, [publicClient, contractAddresses.verifier]);
  
  // Calculate message age when selectedMessage changes
  useEffect(() => {
    if (!selectedMessage || !selectedMessage.timestamp) {
      setMessageAge(0);
      setIsMessageTooOld(false);
      return;
    }
    
    const updateAge = () => {
      const now = Math.floor(Date.now() / 1000);
      const age = now - selectedMessage.timestamp;
      setMessageAge(age);
      setIsMessageTooOld(age > maxQuoteAge);
    };
    
    updateAge();
    
    // Update age every second
    const interval = setInterval(updateAge, 1000);
    return () => clearInterval(interval);
  }, [selectedMessage, maxQuoteAge]);

  // Загрузка актуального сообщения
  useEffect(() => {
    const fetchLatestMessage = async () => {
      try {
        setLoading(true);
        const response = await fetch(`${apiUrl}/data/0/latest`);
        if (!response.ok) {
          throw new Error(`Ошибка: ${response.status}`);
        }
        const data = await response.json();
        setLatestMessage(data);
        setSelectedMessage(data);
        
        // Получаем подписи
        if (data.signatures) {
          const sigList = Object.values(data.signatures);
          setSignatures(sigList);
        }
      } catch (err) {
        console.error('Ошибка при загрузке актуального сообщения:', err);
        setError('Не удалось загрузить актуальное сообщение.');
      } finally {
        setLoading(false);
      }
    };

    fetchLatestMessage();
  }, [apiUrl]);

  // Обработка сообщения через контракт
  const processMessage = async () => {
    if (!selectedMessage || !walletClient || !account || signatures.length === 0) {
      setError('Необходимо выбрать сообщение и подключить кошелек');
      return;
    }
    

    try {
      setLoading(true);
      setError(null);
      setTxHash(null);
      setTxStatus('pending');
      
      // Get the data and timestamp from the message
      const data = JSON.stringify(selectedMessage.data);
      const timestamp = selectedMessage.timestamp;
      
      console.log([
        data,
        timestamp,
        signatures
      ]);

      // Call processPrice with the data, timestamp, and signatures
      const hash = await walletClient.writeContract({
        address: contractAddresses.trading,
        abi: tradingAbi,
        functionName: 'processPrice',
        args: [
          data,
          timestamp,
          signatures
        ],
        account
      });

      setTxHash(hash);
      setTxStatus('processing');

      // Ожидаем получение чека транзакции
      const receipt = await publicClient.waitForTransactionReceipt({ hash });
      
      if (receipt.status === 'success') {
        setTxStatus('success');
      } else {
        setTxStatus('failed');
        setError('Транзакция не выполнена');
      }
    } catch (err) {
      console.error('Ошибка при обработке сообщения:', err);
      setError(`Ошибка при обработке сообщения: ${err.message}`);
      setTxStatus('failed');
    } finally {
      setLoading(false);
    }
  };

  // Загрузка сообщения по хешу
  const loadMessageByHash = async () => {
    const hash = prompt('Введите хеш сообщения:');
    if (!hash) return;
    
    try {
      setLoading(true);
      setError(null);
      const response = await fetch(`${apiUrl}/hash?hash=${hash}`);
      if (!response.ok) {
        throw new Error(`Ошибка: ${response.status}`);
      }
      const data = await response.json();
      setSelectedMessage(data);
      
      // Получаем подписи
      if (data.signatures) {
        const sigList = Object.values(data.signatures);
        setSignatures(sigList);
      } else {
        setSignatures([]);
      }
    } catch (err) {
      console.error('Ошибка при загрузке сообщения:', err);
      setError('Не удалось загрузить сообщение по указанному хешу.');
    } finally {
      setLoading(false);
    }
  };

  // Formatting time functions
  const formatTimeAgo = (seconds) => {
    if (seconds < 60) return `${seconds} сек.`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)} мин. ${seconds % 60} сек.`;
    return `${Math.floor(seconds / 3600)} ч. ${Math.floor((seconds % 3600) / 60)} мин.`;
  };

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-semibold mb-4">Обработка Цены</h2>
      
      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
          {error}
        </div>
      )}
      
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Информация о сообщении */}
        <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-200">
          <div className="flex justify-between items-center mb-3">
            <h3 className="text-lg font-medium">Выбранное сообщение</h3>
            <div className="space-x-2">
              <button
                onClick={() => {
                  setSelectedMessage(latestMessage);
                  setSignatures(Object.values(latestMessage?.signatures || {}));
                }}
                className="px-3 py-1 bg-green-500 text-white text-sm rounded hover:bg-green-600"
                disabled={loading || !latestMessage}
              >
                Актуальное
              </button>
              <button
                onClick={loadMessageByHash}
                className="px-3 py-1 bg-blue-500 text-white text-sm rounded hover:bg-blue-600"
                disabled={loading}
              >
                По хешу
              </button>
            </div>
          </div>
          
          {loading && !selectedMessage && (
            <div className="py-4 text-center">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-500 mx-auto"></div>
              <p className="mt-2 text-gray-500">Загрузка сообщения...</p>
            </div>
          )}
          
          {!selectedMessage ? (
            <div className="py-6 text-center">
              <p className="text-gray-500">Сообщение не выбрано</p>
            </div>
          ) : (
            <div className="space-y-3">
              <div>
                <p className="text-sm font-medium text-gray-600">Хеш:</p>
                <p className="font-mono text-sm break-all bg-gray-50 p-2 rounded">{selectedMessage.hash}</p>
              </div>
              
              <div>
                <p className="text-sm font-medium text-gray-600">Время создания:</p>
                <p className="text-sm">
                  {new Date(selectedMessage.timestamp * 1000).toLocaleString('ru-RU')}
                  {selectedMessage.timestamp && (
                    <span className={`ml-2 ${isMessageTooOld ? 'text-red-600 font-bold' : 'text-gray-500'}`}>
                      ({formatTimeAgo(messageAge)} назад)
                    </span>
                  )}
                </p>
                {isMessageTooOld && (
                  <p className="text-red-600 text-sm mt-1">
                    ⚠️ Сообщение устарело! Максимальный возраст сообщения: {formatTimeAgo(maxQuoteAge)}
                  </p>
                )}
              </div>
              
              <div>
                <p className="text-sm font-medium text-gray-600">Количество подписей:</p>
                <p className="text-sm">{signatures.length}</p>
              </div>
              
              <div>
                <p className="text-sm font-medium text-gray-600">Данные:</p>
                <div className="overflow-x-auto bg-gray-50 p-2 rounded">
                  <pre className="text-xs">{JSON.stringify(selectedMessage.data, null, 2)}</pre>
                </div>
              </div>
            </div>
          )}
        </div>
        
        {/* Форма отправки транзакции */}
        <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-200">
          <h3 className="text-lg font-medium mb-3">Отправка в контракт</h3>
          
          {!account ? (
            <div className="py-4 bg-yellow-50 border border-yellow-100 rounded text-center">
              <p className="text-yellow-700">Подключите кошелек для отправки транзакций</p>
            </div>
          ) : !selectedMessage ? (
            <div className="py-4 bg-gray-50 border border-gray-100 rounded text-center">
              <p className="text-gray-600">Выберите сообщение для обработки</p>
            </div>
          ) : signatures.length === 0 ? (
            <div className="py-4 bg-yellow-50 border border-yellow-100 rounded text-center">
              <p className="text-yellow-700">У выбранного сообщения нет подписей</p>
            </div>
          )  : (
            <div className="space-y-4">
              <p className="text-sm text-gray-600">
                Контракт: <span className="font-mono">{contractAddresses.trading}</span>
              </p>
              <p className="text-sm text-gray-600">
                Функция: <span className="font-mono">processPrice(string data, uint256 timestamp, bytes[] signatures)</span>
              </p>
              <p className="text-sm text-gray-600">
                Количество подписей: <span className="font-mono">{signatures.length}</span>
              </p>
              
              <button
                onClick={processMessage}
                disabled={loading || !walletClient || !selectedMessage || signatures.length === 0}
                className={`w-full py-2 px-4 rounded font-medium ${
                  loading
                    ? 'bg-gray-400 cursor-not-allowed'
                    : 'bg-blue-600 text-white hover:bg-blue-700'
                }`}
              >
                {loading ? 'Обработка...' : 'Отправить транзакцию'}
              </button>
              
              {txHash && (
                <div className={`p-3 rounded ${
                  txStatus === 'success' 
                    ? 'bg-green-50 border border-green-100' 
                    : txStatus === 'failed'
                    ? 'bg-red-50 border border-red-100'
                    : 'bg-blue-50 border border-blue-100'
                }`}>
                  <p className={`text-sm mb-1 ${
                    txStatus === 'success' 
                      ? 'text-green-700' 
                      : txStatus === 'failed'
                      ? 'text-red-700'
                      : 'text-blue-700'
                  }`}>
                    {txStatus === 'success' 
                      ? '✅ Транзакция успешно выполнена' 
                      : txStatus === 'failed'
                      ? '❌ Транзакция не выполнена'
                      : '⏳ Транзакция в обработке'}
                  </p>
                  <p className="text-xs break-all">
                    <a 
                      href={`https://testnet.bscscan.com/tx/${txHash}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-indigo-600 hover:underline"
                    >
                      {txHash}
                    </a>
                  </p>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
      
      {/* Пояснения */}
      <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-200">
        <h3 className="text-lg font-medium mb-3">О процессе проверки цены</h3>
        
        <div className="space-y-4 text-sm text-gray-700">
          <p>
            В этом разделе вы можете отправить подписанное сообщение с ценой в смарт-контракт 
            <span className="font-mono text-sm ml-1">AssetTradingWithQuotes</span>, 
            который затем выполнит его проверку с помощью контракта 
            <span className="font-mono text-sm ml-1">OracleVerifier</span>.
          </p>
          
          <ol className="list-decimal pl-5 space-y-2">
            <li>
              <strong>Выберите сообщение</strong> - Используйте актуальное сообщение или найдите по хешу
            </li>
            <li>
              <strong>Проверьте подписи</strong> - Убедитесь, что у сообщения есть достаточное количество подписей
            </li>
            <li>
              <strong>Проверьте возраст</strong> - Сообщение не должно быть старше {formatTimeAgo(maxQuoteAge)}
            </li>
            <li>
              <strong>Отправьте транзакцию</strong> - Нажмите кнопку для отправки сообщения в контракт
            </li>
            <li>
              <strong>Проверьте результат</strong> - После обработки транзакции будет сгенерировано событие:
              <ul className="list-disc pl-5 mt-1">
                <li><span className="font-mono text-xs">PriceVerified</span> - если проверка прошла успешно</li>
                <li><span className="font-mono text-xs">PriceRejected</span> - если проверка не удалась</li>
              </ul>
            </li>
          </ol>
          
          <div className="bg-blue-50 border border-blue-100 rounded-md p-3">
            <p className="text-blue-700">
              <span className="font-bold">Важно:</span> Для успешной проверки количество валидных подписей должно быть 
              не меньше порогового значения (threshold) в контракте OracleVerifier. Также сообщение не должно быть
              старше {formatTimeAgo(maxQuoteAge)}. Текущие параметры можно увидеть в разделе "Информация о Контрактах".
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};

export default PriceProcessor; 