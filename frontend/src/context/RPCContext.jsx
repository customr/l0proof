import { createContext, useContext, useState, useCallback, useRef } from 'react'

const RPCContext = createContext()

// Cache expiration time in milliseconds
const CACHE_EXPIRY = 10000 // 10 seconds
// Default debounce time
const DEFAULT_DEBOUNCE_TIME = 500 // 500ms

const API_URL = import.meta.env.REACT_APP_API_URL || "http://localhost:8080"

// API endpoints
const ENDPOINTS = {
  QUOTES_LATEST: '/quotes/latest',
  HASH: '/hash',
  DATA_STRUCTURE: '/data',
  STRUCTURES: '/structures',
  SIGNED_QUOTE: '/signed_quote',
  HEALTH: '/health'
}

export function RPCProvider({ children }) {
  const [apiUrl, setApiUrl] = useState(API_URL)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const cacheRef = useRef({})
  const pendingRequestsRef = useRef({})

  const getCacheKey = (endpoint, params) => {
    return `${endpoint}?${new URLSearchParams(params).toString()}`
  }

  const fetchData = useCallback(async (endpoint, params = {}, options = {}) => {
    const { skipCache = false, debounceTime = DEFAULT_DEBOUNCE_TIME } = options
    const cacheKey = getCacheKey(endpoint, params)
    
    // Return cached data if available and not expired
    if (!skipCache && cacheRef.current[cacheKey]) {
      const { data, timestamp } = cacheRef.current[cacheKey]
      if (Date.now() - timestamp < CACHE_EXPIRY) {
        return data
      }
    }

    // Debounce the request
    if (pendingRequestsRef.current[cacheKey]) {
      clearTimeout(pendingRequestsRef.current[cacheKey].timeoutId)
    }

    return new Promise((resolve, reject) => {
      pendingRequestsRef.current[cacheKey] = {
        timeoutId: setTimeout(async () => {
          setLoading(true)
          setError(null)
          
          try {
            const url = new URL(endpoint, apiUrl)
            Object.keys(params).forEach(key => 
              url.searchParams.append(key, params[key])
            )
            
            const response = await fetch(url.toString())
            if (!response.ok) {
              const errorData = await response.json().catch(() => ({}))
              throw new Error(errorData.message || `HTTP error! status: ${response.status}`)
            }
            const data = await response.json()
            
            // Cache the response
            cacheRef.current[cacheKey] = {
              data,
              timestamp: Date.now()
            }
            
            resolve(data)
          } catch (err) {
            setError(err.message)
            reject(err)
          } finally {
            setLoading(false)
            delete pendingRequestsRef.current[cacheKey]
          }
        }, debounceTime)
      }
    })
  }, [apiUrl])

  // API methods
  const api = {
    // Get latest quotes for all assets
    getLatestQuotes: () => 
      fetchData(ENDPOINTS.QUOTES_LATEST, {}, { skipCache: true }),

    // Get message by hash
    getMessageByHash: (hash) => 
      fetchData(ENDPOINTS.HASH, { hash }),

    // Get data structure field values
    getFieldValues: (dataStructureId, field) => 
      fetchData(`${ENDPOINTS.DATA_STRUCTURE}/${dataStructureId}/${field}`),

    // Get latest message for a data structure
    getLatest: (dataStructureId, field = null, value = null) => {
      const params = {};
      if (field && value) {
        params.field = field;
        params.value = value;
      }
      return fetchData(`${ENDPOINTS.DATA_STRUCTURE}/${dataStructureId}/latest`, params, { skipCache: true });
    },

    // Get all data structures
    getStructures: () => 
      fetchData(ENDPOINTS.STRUCTURES),
      
    // Get signed price quote for an asset
    getSignedQuote: (asset) => 
      fetchData(ENDPOINTS.SIGNED_QUOTE, { asset }, { skipCache: true }),
      
    // Check health of the API service
    checkHealth: () =>
      fetchData(ENDPOINTS.HEALTH, {}, { skipCache: true })
  }

  // Function to invalidate cache (useful after mutations)
  const invalidateCache = useCallback((endpoint = null) => {
    if (endpoint) {
      // Invalidate specific endpoint cache
      Object.keys(cacheRef.current).forEach(key => {
        if (key.startsWith(endpoint)) {
          delete cacheRef.current[key]
        }
      })
    } else {
      // Invalidate all cache
      cacheRef.current = {}
    }
  }, [])

  return (
    <RPCContext.Provider value={{ 
      apiUrl, 
      setApiUrl, 
      api,
      loading, 
      error, 
      invalidateCache 
    }}>
      {children}
    </RPCContext.Provider>
  )
}

export const useRPC = () => useContext(RPCContext)