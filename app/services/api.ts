import axios from 'axios';
import { API_CONFIG } from '../config/api.config';

const API_BASE_URL = API_CONFIG.BASE_URL;

console.log('🔧 API Configuration Loaded:', {
  baseURL: API_BASE_URL,
  isDev: __DEV__,
  config: API_CONFIG,
});

export interface LeaderboardEntry {
  rank: number;
  username: string;
  rating: number;
}

export interface SearchResult {
  id: string;
  username: string;
  rating: number;
  rank: number;
}

export interface SearchResponse {
  data: SearchResult[];
  count: number;
  query: string;
}

class LeaderboardAPI {
  private baseURL: string;

  constructor(baseURL: string = API_BASE_URL) {
    this.baseURL = baseURL;
  }

  /**
   * Fetch leaderboard with optional limit and offset
   * @param limit Number of entries to fetch (default: 100)
   * @param offset Offset for pagination (default: 0)
   */
  async getLeaderboard(limit: number = 100, offset: number = 0): Promise<LeaderboardEntry[]> {
    try {
      const response = await axios.get<LeaderboardEntry[]>(
        `${this.baseURL}/leaderboard`,
        {
          params: { limit, offset },
          timeout: 10000, // 10 second timeout for debugging
        }
      );
      
      return response.data || [];
    } catch (error) {
      if (axios.isAxiosError(error)) {
        console.error('Axios Error Details:', {
          message: error.message,
          code: error.code,
          url: error.config?.url,
          baseURL: this.baseURL,
          response: error.response?.status,
        });
      } else {
        console.error('Failed to fetch leaderboard:', error);
      }
      throw error;
    }
  }

  /**
   * Search users by username
   * @param query Search query string
   */
  async searchUsers(query: string): Promise<SearchResult[]> {
    try {
      if (!query || query.trim().length === 0) {
        return [];
      }

      const response = await axios.get<SearchResponse>(
        `${this.baseURL}/search`,
        {
          params: { query: query },
          timeout: 5000,
        }
      );
      return response.data.data || [];
    } catch (error) {
      console.error('Failed to search users:', error);
      throw error;
    }
  }

  /**
   * Health check endpoint
   */
  async healthCheck(): Promise<boolean> {
    try {
      await axios.get(`${this.baseURL}/health`, { timeout: 2000 });
      return true;
    } catch (error) {
      return false;
    }
  }
}

export const leaderboardAPI = new LeaderboardAPI();
