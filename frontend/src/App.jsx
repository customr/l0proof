import { useState, useEffect } from 'react'
import { HashRouter, Routes, Route, Navigate } from 'react-router-dom'
import Navbar from './components/Navbar'
import OracleVerifier from './pages/OracleVerifier'
import NotFound from './pages/NotFound'
import { RPCProvider } from './context/RPCContext'

function App() {
  return (
    <RPCProvider>
      <HashRouter>
        <div className="min-h-screen bg-gray-50">
          <Navbar />
          <div className="container mx-auto px-4 py-8">
            <Routes>
              <Route path="/" element={<OracleVerifier />} />
              <Route path="*" element={<NotFound />} />
            </Routes>
          </div>
        </div>
      </HashRouter>
    </RPCProvider>
  )
}

export default App