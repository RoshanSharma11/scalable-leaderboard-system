import React, { useState, useCallback, useEffect, useRef } from 'react';
import { StyleSheet, View, TextInput, ActivityIndicator, Text, RefreshControl, TouchableOpacity, Keyboard } from 'react-native';
import { FlashList } from '@shopify/flash-list';
import { SafeAreaView } from 'react-native-safe-area-context';
import { Ionicons } from '@expo/vector-icons';
import { StatusBar } from 'expo-status-bar';
import LeaderboardItem from '../../components/LeaderboardItem';
import { leaderboardAPI, type LeaderboardEntry } from '../../services/api';
import { API_CONFIG, PERFORMANCE_CONFIG } from '../../config/api.config';

const REFRESH_INTERVAL = API_CONFIG.REFRESH_INTERVAL;

export default function LeaderboardScreen() {
  const [data, setData] = useState<LeaderboardEntry[]>([]);
  const [filteredData, setFilteredData] = useState<LeaderboardEntry[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [searching, setSearching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isLiveMode, setIsLiveMode] = useState(true);

  const searchTimeoutRef = useRef<number | null>(null);
  const liveUpdateIntervalRef = useRef<number | null>(null);

  const fetchLeaderboard = useCallback(async (showLoading = true) => {
    try {
      if (showLoading) setLoading(true);
      setError(null);

      const leaderboard = await leaderboardAPI.getLeaderboard(API_CONFIG.INITIAL_LOAD_COUNT);
      
      const sortedLeaderboard = leaderboard.sort((a, b) => {
        if (a.rank !== b.rank) return a.rank - b.rank;
        return a.username.localeCompare(b.username);
      });
      
      setData(sortedLeaderboard);
      if (!searchQuery) setFilteredData(sortedLeaderboard);
    } catch (err) {
      setError('Failed to load leaderboard');
      console.error('Fetch error:', err);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [searchQuery]);

  const performSearch = useCallback(async (query: string) => {
    if (!query || query.trim().length === 0) {
      setFilteredData(data);
      setSearching(false);
      return;
    }

    try {
      setSearching(true);
      const results = await leaderboardAPI.searchUsers(query);
      
      const searchData: LeaderboardEntry[] = results.map((result) => ({
        rank: result.rank,
        username: result.username,
        rating: result.rating,
      }));

      setFilteredData(searchData);
    } catch (err) {
      console.error('Search error:', err);
      const filtered = data.filter((item) =>
        item.username.toLowerCase().includes(query.toLowerCase())
      );
      setFilteredData(filtered);
    } finally {
      setSearching(false);
    }
  }, [data]);

  const handleSearchChange = useCallback((text: string) => {
    setSearchQuery(text);
    if (searchTimeoutRef.current) clearTimeout(searchTimeoutRef.current);
    searchTimeoutRef.current = setTimeout(() => performSearch(text), PERFORMANCE_CONFIG.SEARCH_DEBOUNCE_DELAY);
  }, [performSearch]);

  const clearSearch = useCallback(() => {
    setSearchQuery('');
    setFilteredData(data);
    Keyboard.dismiss();
  }, [data]);

  const handleRefresh = useCallback(() => {
    setRefreshing(true);
    fetchLeaderboard(false);
  }, [fetchLeaderboard]);

  const toggleLiveMode = useCallback(() => {
    setIsLiveMode((prev) => !prev);
  }, []);

  useEffect(() => {
    fetchLeaderboard();
  }, [fetchLeaderboard]);

  useEffect(() => {
    if (!isLiveMode) {
      if (liveUpdateIntervalRef.current) clearInterval(liveUpdateIntervalRef.current);
      return;
    }

    liveUpdateIntervalRef.current = setInterval(() => {
      fetchLeaderboard(false);
    }, REFRESH_INTERVAL);

    return () => {
      if (liveUpdateIntervalRef.current) clearInterval(liveUpdateIntervalRef.current);
    };
  }, [isLiveMode, fetchLeaderboard]);

  useEffect(() => {
    return () => {
      if (searchTimeoutRef.current) clearTimeout(searchTimeoutRef.current);
      if (liveUpdateIntervalRef.current) clearInterval(liveUpdateIntervalRef.current);
    };
  }, []);

  const renderItem = useCallback(
    ({ item, index }: { item: LeaderboardEntry; index: number }) => (
      <LeaderboardItem item={item} index={index} />
    ),
    []
  );

  const keyExtractor = useCallback(
    (item: LeaderboardEntry, index: number) => `${item.rank}-${item.username}-${index}`,
    []
  );

  const getItemType = useCallback(
    (item: LeaderboardEntry) => (item.rank <= 3 ? 'top' : 'normal'),
    []
  );

  const renderEmptyComponent = useCallback(() => {
    if (loading) return null;

    return (
      <View style={styles.emptyContainer}>
        <Text style={styles.emptyText}>
          {searchQuery ? 'No users found' : 'No leaderboard data'}
        </Text>
      </View>
    );
  }, [loading, searchQuery]);

  const renderListHeader = useCallback(() => {
    if (searchQuery || filteredData.length === 0) return null;

    return (
      <View style={styles.headerContainer}>
        <Text style={styles.headerText}>
          {filteredData.length.toLocaleString()} Players
        </Text>
        {isLiveMode && (
          <View style={styles.liveIndicator}>
            <View style={styles.liveDot} />
            <Text style={styles.liveText}>LIVE</Text>
          </View>
        )}
      </View>
    );
  }, [searchQuery, filteredData.length, isLiveMode]);

  if (loading && data.length === 0) {
    return (
      <View style={styles.loadingContainer}>
        <ActivityIndicator size="large" color="#1a73e8" />
        <Text style={styles.loadingText}>Loading Leaderboard...</Text>
      </View>
    );
  }

  if (error && data.length === 0) {
    return (
      <View style={styles.errorContainer}>
        <Text style={styles.errorText}>{error}</Text>
        <TouchableOpacity style={styles.retryButton} onPress={() => fetchLeaderboard()}>
          <Text style={styles.retryButtonText}>Retry</Text>
        </TouchableOpacity>
      </View>
    );
  }

  return (
    <SafeAreaView style={styles.safeArea}>
      <View style={styles.container}>
        <StatusBar style="dark" />

        <View style={styles.searchContainer}>
          <View style={styles.searchInputContainer}>
            <Ionicons name="search" size={18} color="#666" />
            <TextInput
              style={styles.searchInput}
              placeholder="Search players..."
              placeholderTextColor="#999"
              value={searchQuery}
              onChangeText={handleSearchChange}
              autoCapitalize="none"
              autoCorrect={false}
            />
            {searching && <ActivityIndicator size="small" color="#1a73e8" />}
            {searchQuery.length > 0 && !searching && (
              <TouchableOpacity onPress={clearSearch} style={styles.clearButton}>
                <Ionicons name="close-circle" size={18} color="#999" />
              </TouchableOpacity>
            )}
          </View>

          <TouchableOpacity
            style={[styles.liveButton, isLiveMode && styles.liveButtonActive]}
            onPress={toggleLiveMode}
          >
            <Text style={[styles.liveButtonText, isLiveMode && styles.liveButtonTextActive]}>
              {isLiveMode ? '⚡ LIVE' : 'PAUSED'}
            </Text>
          </TouchableOpacity>
        </View>

        <FlashList
          data={filteredData}
          renderItem={renderItem}
          keyExtractor={keyExtractor}
          getItemType={getItemType}
          ListHeaderComponent={renderListHeader}
          ListEmptyComponent={renderEmptyComponent}
          refreshControl={
            <RefreshControl
              refreshing={refreshing}
              onRefresh={handleRefresh}
              tintColor="#1a73e8"
              colors={['#1a73e8']}
            />
          }
          showsVerticalScrollIndicator={true}
        />
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safeArea: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  container: {
    flex: 1,
  },
  loadingContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#f5f5f5',
  },
  loadingText: {
    marginTop: 12,
    fontSize: 16,
    color: '#666',
  },
  errorContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#f5f5f5',
    paddingHorizontal: 20,
  },
  errorText: {
    fontSize: 16,
    color: '#d32f2f',
    textAlign: 'center',
    marginBottom: 20,
  },
  retryButton: {
    backgroundColor: '#1a73e8',
    paddingHorizontal: 24,
    paddingVertical: 12,
    borderRadius: 8,
  },
  retryButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
  searchContainer: {
    flexDirection: 'row',
    paddingHorizontal: 12,
    paddingVertical: 12,
    backgroundColor: '#fff',
    borderBottomWidth: 1,
    borderBottomColor: '#e0e0e0',
  },
  searchInputContainer: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#f5f5f5',
    borderRadius: 8,
    paddingHorizontal: 12,
    marginRight: 8,
  },
  searchInput: {
    flex: 1,
    height: 40,
    fontSize: 16,
    color: '#333',
    marginLeft: 8,
  },
  clearButton: {
    padding: 4,
  },
  liveButton: {
    backgroundColor: '#e0e0e0',
    paddingHorizontal: 12,
    paddingVertical: 8,
    borderRadius: 8,
    justifyContent: 'center',
    alignItems: 'center',
    minWidth: 70,
  },
  liveButtonActive: {
    backgroundColor: '#4caf50',
  },
  liveButtonText: {
    fontSize: 12,
    fontWeight: '700',
    color: '#666',
  },
  liveButtonTextActive: {
    color: '#fff',
  },
  headerContainer: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: 16,
    backgroundColor: '#fff',
    borderBottomWidth: 2,
    borderBottomColor: '#1a73e8',
  },
  headerText: {
    fontSize: 18,
    fontWeight: '700',
    color: '#333',
  },
  liveIndicator: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#e8f5e9',
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: 4,
  },
  liveDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
    backgroundColor: '#4caf50',
    marginRight: 6,
  },
  liveText: {
    fontSize: 11,
    fontWeight: '700',
    color: '#2e7d32',
  },
  emptyContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    paddingVertical: 40,
  },
  emptyText: {
    fontSize: 16,
    color: '#999',
  },
});
