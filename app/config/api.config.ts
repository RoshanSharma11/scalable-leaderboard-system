const getApiUrl = () => {
  // Priority: Environment variable > Development default > Production placeholder
  if (process.env.EXPO_PUBLIC_API_URL) {
    return process.env.EXPO_PUBLIC_API_URL;
  }
  
  if (__DEV__) {
    return 'http://192.168.1.2:8000';
  }
  
  return 'https://your-production-api.com';
};

export const API_CONFIG = {
  BASE_URL: getApiUrl(),
  
  // Polling interval for live updates (milliseconds)
  REFRESH_INTERVAL: 2000,
  
  // Initial number of leaderboard entries to load
  INITIAL_LOAD_COUNT: 1000,
  
  // Request timeout (milliseconds)
  TIMEOUT: 5000,
};

export const PERFORMANCE_CONFIG = {
  // FlashList optimization
  ESTIMATED_ITEM_SIZE: 56,
  MAX_TO_RENDER_PER_BATCH: 20,
  UPDATE_CELLS_BATCHING_PERIOD: 50,
  WINDOW_SIZE: 10,
  
  // Search debounce delay (milliseconds)
  // Reduced to 100ms since n-gram search is extremely fast (< 1 ms)
  SEARCH_DEBOUNCE_DELAY: 100,
};
