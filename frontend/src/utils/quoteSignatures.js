import { ethers } from 'ethers';

/**
 * Utility class to manage price quote signatures for the provable price oracle system
 */
export class SignedQuoteManager {
  /**
   * @param {string} apiUrl - Base URL for the oracle API
   */
  constructor(apiUrl) {
    this.apiUrl = apiUrl.endsWith('/') ? apiUrl.slice(0, -1) : apiUrl;
  }

  /**
   * Fetches a signed quote for a specific asset from the API
   * @param {string} asset - Asset symbol (e.g. "SBER", "BTC", etc.)
   * @returns {Promise<Object>} - The signed quote with all signatures
   */
  async fetchSignedQuote(asset) {
    try {
      const response = await fetch(`${this.apiUrl}/signed_quote?asset=${encodeURIComponent(asset)}`);
      
      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`API error (${response.status}): ${errorText}`);
      }
      
      const data = await response.json();
      return data;
    } catch (error) {
      console.error('Error fetching signed quote:', error);
      throw error;
    }
  }

  /**
   * Prepares a quote for use in the smart contract by formatting signatures
   * @param {string} asset - Asset symbol
   * @returns {Object} - Quote data formatted for contract calls
   */
  async getQuoteForContract(asset) {
    const quoteData = await this.fetchSignedQuote(asset);
    
    // Store original data for reference
    const result = {
      asset: quoteData.asset,
      price: quoteData.price,
      timestamp: quoteData.timestamp,
      signers: [],
      signatures: [],
      originalData: quoteData
    };
    
    // Process signatures into arrays needed for contract
    if (quoteData.signatures) {
      for (const [signer, signature] of Object.entries(quoteData.signatures)) {
        const sig = ethers.hexlify(signature);
        
        // Add to arrays
        result.signers.push(signer);
        result.signatures.push(sig);
      }
    }
    
    return result;
  }
  
  /**
   * Checks if a quote is still valid based on its timestamp
   * @param {Object} quote - The quote to check
   * @param {number} maxAgeSeconds - Maximum allowed age in seconds
   * @returns {boolean} - Whether the quote is still valid
   */
  isQuoteValid(quote, maxAgeSeconds) {
    if (!quote || !quote.timestamp) return false;
    
    const now = Math.floor(Date.now() / 1000);
    const quoteTime = quote.timestamp;
    const ageSeconds = now - quoteTime;
    
    return ageSeconds <= maxAgeSeconds;
  }
  
  /**
   * Gets the signatures and current timestamp of a quote
   * @param {Object} quote - The quote to format 
   * @returns {Object} - Formatted data for the contract call
   */
  formatQuoteForContract(quote) {
    return {
      asset: quote.asset,
      price: quote.price,
      timestamp: quote.timestamp,
      signatures: quote.signatures,
      signers: quote.signers
    };
  }
} 