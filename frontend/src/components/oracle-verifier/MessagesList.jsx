import React, { useState, useEffect } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { ru } from 'date-fns/locale';

const MessagesList = ({ apiUrl, onSelectMessage }) => {
  const [messages, setMessages] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [page, setPage] = useState(0);
  const [limit] = useState(10);
  const [hashFilter, setHashFilter] = useState('');
  const [timestampFilter, setTimestampFilter] = useState('');
  
  const [fieldName, setFieldName] = useState('');
  const [fieldValue, setFieldValue] = useState('');
  
  const [isFiltering, setIsFiltering] = useState(false);

  const fetchMessages = async () => {
    setLoading(true);
    setError(null);
    try {
      let url;
      if (hashFilter) {
        url = `${apiUrl}/hash?hash=${hashFilter}`;
        setIsFiltering(true);
      } else if (timestampFilter) {
        url = `${apiUrl}/data/0/list?timestamp=${timestampFilter}&page=${page+1}&limit=${limit}`;
        setIsFiltering(true);
      } else if (fieldName && fieldValue) {
        // Search by any custom field
        url = `${apiUrl}/data/0/list?${fieldName}=${fieldValue}&page=${page+1}&limit=${limit}`;
        setIsFiltering(true);
      } else {
        url = `${apiUrl}/list?page=${page+1}&limit=${limit}`;
        setIsFiltering(false);
      }
      
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error(`Ошибка: ${response.status}`);
      }
      
      const data = await response.json();
      
      if (hashFilter && !Array.isArray(data)) {
        // Handle single message result from hash search
        setMessages([data]);
      } else {
        setMessages(Array.isArray(data) ? data : []);
      }
    } catch (err) {
      console.error('Ошибка при загрузке сообщений:', err);
      setError('Не удалось загрузить сообщения. Пожалуйста, проверьте соединение.');
    } finally {
      setLoading(false);
    }
  };
  
  useEffect(() => {
    fetchMessages();
    if (!hashFilter && !timestampFilter && !fieldName && !fieldValue) {
      loadLatestMessage();
    }
  }, [apiUrl, page, limit, hashFilter, timestampFilter, fieldName, fieldValue]);

  const loadMessageDetails = async (hash) => {
    try {
      setLoading(true);
      const response = await fetch(`${apiUrl}/hash?hash=${hash}`);
      if (!response.ok) {
        throw new Error(`Ошибка: ${response.status}`);
      }
      const data = await response.json();
      onSelectMessage(data);
    } catch (err) {
      console.error('Ошибка при загрузке деталей сообщения:', err);
      setError('Не удалось загрузить детали сообщения.');
    } finally {
      setLoading(false);
    }
  };

  const loadLatestMessage = async () => {
    try {
      setLoading(true);
      fetchMessages();
      const response = await fetch(`${apiUrl}/data/0/latest`);
      if (!response.ok) {
        throw new Error(`Ошибка: ${response.status}`);
      }
      const data = await response.json();
      setMessages(prevMessages => {
        // Проверяем, есть ли уже сообщение с таким хешем
        const exists = prevMessages.some(msg => msg.hash === data.hash);
        if (!exists) {
          return [data, ...prevMessages];
        }
        return prevMessages;
      });
      onSelectMessage(data);
    } catch (err) {
      console.error('Ошибка при загрузке актуального сообщения:', err);
      setError('Не удалось загрузить актуальное сообщение.');
    } finally {
      setLoading(false);
    }
  };

  const resetFilters = () => {
    setHashFilter('');
    setTimestampFilter('');
    setFieldName('');
    setFieldValue('');
    setPage(0);
    setIsFiltering(false);
  };

  const applyFieldSearch = (e) => {
    e.preventDefault();
    fetchMessages();
  };

  const formatTime = (timestamp) => {
    const date = new Date(timestamp * 1000);
    return `${date.toLocaleDateString('ru-RU')} ${date.toLocaleTimeString('ru-RU')}`;
  };

  return (
    <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-200">
      <div className="flex justify-between items-center mb-3">
        <h3 className="text-lg font-medium">Сообщения от нод</h3>
        <button
          onClick={loadLatestMessage}
          className="px-3 py-1 bg-green-500 text-white text-sm rounded hover:bg-green-600"
        >
          Обновить
        </button>
      </div>
      
      <div className="mb-4 space-y-3">
        <div className="grid grid-cols-1 gap-2">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Поиск по хешу:</label>
            <input
              type="text"
              value={hashFilter}
              onChange={(e) => {
                setHashFilter(e.target.value);
                setTimestampFilter('');
                setFieldName('');
                setFieldValue('');
              }}
              placeholder="Введите хеш сообщения..."
              className="w-full p-2 border rounded"
            />
          </div>
          
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Поиск по времени (Unix timestamp):</label>
            <input
              type="text"
              value={timestampFilter}
              onChange={(e) => {
                setTimestampFilter(e.target.value);
                setHashFilter('');
                setFieldName('');
                setFieldValue('');
              }}
              placeholder="Введите timestamp..."
              className="w-full p-2 border rounded"
            />
          </div>
        </div>
        
        <div className="pt-1 border-t border-gray-100">
          <label className="block text-sm font-medium text-gray-700 mb-1">Поиск по любому полю:</label>
          <form onSubmit={applyFieldSearch} className="flex space-x-2">
            <input
              type="text"
              value={fieldName}
              onChange={(e) => setFieldName(e.target.value)}
              placeholder="Имя поля"
              className="flex-1 p-2 border rounded"
            />
            <input
              type="text"
              value={fieldValue}
              onChange={(e) => setFieldValue(e.target.value)}
              placeholder="Значение"
              className="flex-1 p-2 border rounded"
            />
            <button
              type="submit"
              className="px-3 py-1 bg-blue-500 text-white text-sm rounded hover:bg-blue-600"
            >
              Найти
            </button>
          </form>
        </div>
        
        {isFiltering && (
          <div className="text-right pt-1">
            <button
              onClick={resetFilters}
              className="px-3 py-1 text-sm text-red-600 rounded hover:bg-red-50"
            >
              Сбросить фильтры
            </button>
          </div>
        )}
      </div>
      
      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-4">
          {error}
        </div>
      )}
      
      {loading && messages.length === 0 && (
        <div className="py-4 text-center">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-500 mx-auto"></div>
          <p className="mt-2 text-gray-500">Загрузка сообщений...</p>
        </div>
      )}
      
      {!loading && messages.length === 0 && (
        <div className="py-6 text-center">
          <p className="text-gray-500">Нет доступных сообщений</p>
        </div>
      )}
      
      <ul className="divide-y divide-gray-200 overflow-y-auto max-h-96">
        {messages.map((message) => (
          <li key={message.hash} className="py-3">
            <button
              className="w-full text-left px-3 py-2 rounded transition-colors hover:bg-gray-50"
              onClick={() => loadMessageDetails(message.hash)}
            >
              <div className="flex justify-between items-start">
                <div className="truncate flex-1">
                  <p className="font-medium">{message.hash.substring(0, 10)}...{message.hash.substring(message.hash.length - 6)}</p>
                  <p className="text-sm text-gray-500">
                    {formatTime(message.timestamp)}
                  </p>
                </div>
                <div className="ml-2 flex-shrink-0">
                  <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                    Object.keys(message.signatures || {}).length > 0
                      ? 'bg-green-100 text-green-800'
                      : 'bg-yellow-100 text-yellow-800'
                  }`}>
                    {Object.keys(message.signatures || {}).length} подписей
                  </span>
                </div>
              </div>
            </button>
          </li>
        ))}
      </ul>
      
      {/* Пагинация */}
      {!isFiltering && messages.length > 0 && (
        <div className="flex justify-between items-center mt-4 pt-3 border-t border-gray-100">
          <button
            className="px-3 py-1 text-sm text-indigo-600 rounded hover:bg-indigo-50 disabled:opacity-50 disabled:cursor-not-allowed"
            onClick={() => setPage((p) => Math.max(0, p - 1))}
            disabled={page === 0}
          >
            Предыдущая
          </button>
          <span className="text-sm text-gray-500">Страница {page + 1}</span>
          <button
            className="px-3 py-1 text-sm text-indigo-600 rounded hover:bg-indigo-50"
            onClick={() => setPage((p) => p + 1)}
            disabled={messages.length < limit}
          >
            Следующая
          </button>
        </div>
      )}
    </div>
  );
};

export default MessagesList; 