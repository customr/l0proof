import React from 'react';
import { formatDistanceToNow } from 'date-fns';
import { ru } from 'date-fns/locale';

const MessageDetail = ({ message }) => {
  if (!message) {
    return (
      <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-200 h-full flex items-center justify-center">
        <p className="text-gray-500">Выберите сообщение для просмотра деталей</p>
      </div>
    );
  }

  const formatTime = (timestamp) => {
    const date = new Date(timestamp * 1000);
    return `${date.toLocaleDateString('ru-RU')} ${date.toLocaleTimeString('ru-RU')} (${formatDistanceToNow(date, { addSuffix: true, locale: ru })})`;
  };

  const signers = message.signatures ? Object.keys(message.signatures) : [];

  const formatDataField = (value) => {
    if (typeof value === 'object' && value !== null) {
      return JSON.stringify(value);
    }
    return String(value);
  };

  return (
    <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-200 h-full overflow-auto">
      <h3 className="text-lg font-medium mb-4">Детали сообщения</h3>
      
      <div className="space-y-4">
        <div>
          <p className="text-sm font-medium text-gray-600 mb-1">Хеш сообщения:</p>
          <p className="font-mono text-sm break-all bg-gray-50 p-2 rounded">{message.hash}</p>
        </div>
        
        <div>
          <p className="text-sm font-medium text-gray-600 mb-1">Время создания:</p>
          <p className="text-sm">{formatTime(message.timestamp)}</p>
        </div>
        
        <div>
          <p className="text-sm font-medium text-gray-600 mb-1">Данные сообщения:</p>
          <div className="bg-gray-50 p-2 rounded overflow-x-auto">
            <table className="min-w-full text-xs">
              <thead>
                <tr className="border-b border-gray-200">
                  <th className="text-left p-1 font-medium">Поле</th>
                  <th className="text-left p-1 font-medium">Значение</th>
                  <th className="text-left p-1 font-medium">Тип данных</th>
                </tr>
              </thead>
              <tbody>
                {message.data_structure_meta && message.data_structure_meta.map((fieldName, index) => (
                  <tr key={index} className="border-b border-gray-100">
                    <td className="p-1 font-medium">{fieldName}</td>
                    <td className="p-1 font-mono">
                      {message.data && index < message.data.length ? formatDataField(message.data[index]) : 'N/A'}
                    </td>
                    <td className="p-1 text-gray-500">
                      {message.data_structure && index < message.data_structure.length ? message.data_structure[index] : 'N/A'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
        
        <div>
          <p className="text-sm font-medium text-gray-600 mb-1">
            Подписи ({signers.length}):
          </p>
          {signers.length === 0 ? (
            <p className="text-sm text-yellow-600 bg-yellow-50 p-2 rounded">У сообщения нет подписей</p>
          ) : (
            <div className="space-y-2">
              {signers.map((signer, index) => (
                <div key={signer} className="bg-gray-50 p-2 rounded">
                  <p className="text-xs font-bold mb-1">Оракул {index + 1}:</p>
                  <p className="font-mono text-xs break-all">{signer}</p>
                  <p className="text-xs font-medium mt-1 mb-1">Подпись:</p>
                  <p className="font-mono text-xs break-all text-gray-500">{message.signatures[signer]}</p>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default MessageDetail; 