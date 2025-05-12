import { Link } from 'react-router-dom'

export default function NotFound() {
  return (
    <div className="text-center py-20">
      <h1 className="text-4xl font-bold text-gray-800 mb-4">404</h1>
      <h2 className="text-2xl text-gray-600 mb-8">Страница не найдена</h2>
      <p className="text-gray-500 mb-8">
        Запрашиваемая страница не существует или была перемещена.
      </p>
      <Link 
        to="/"
        className="px-6 py-3 bg-indigo-600 text-white rounded-md hover:bg-indigo-700 transition-colors"
      >
        Вернуться на главную
      </Link>
    </div>
  )
} 