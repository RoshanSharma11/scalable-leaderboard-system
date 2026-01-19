import React, { memo } from 'react';
import { StyleSheet, View, Text } from 'react-native';
import type { LeaderboardEntry } from '../services/api';

interface LeaderboardItemProps {
  item: LeaderboardEntry;
  index: number;
}

const LeaderboardItem: React.FC<LeaderboardItemProps> = memo(({ item, index }) => {
  const isTopThree = item.rank <= 3;

  return (
    <View style={[styles.container, index % 2 === 0 && styles.evenRow]}>
      <View style={styles.rankContainer}>
        <Text style={[styles.rank, isTopThree && styles.topRank]}>
          #{item.rank}
        </Text>
      </View>

      <View style={styles.userContainer}>
        <Text style={[styles.username, isTopThree && styles.topUsername]} numberOfLines={1}>
          {item.username}
        </Text>
      </View>

      <View style={styles.ratingContainer}>
        <Text style={[styles.rating, isTopThree && styles.topRating]}>
          {item.rating.toLocaleString()}
        </Text>
      </View>
    </View>
  );
});

LeaderboardItem.displayName = 'LeaderboardItem';

const styles = StyleSheet.create({
  container: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 12,
    paddingHorizontal: 16,
    backgroundColor: '#fff',
    borderBottomWidth: 1,
    borderBottomColor: '#e0e0e0',
  },
  evenRow: {
    backgroundColor: '#f8f9fa',
  },
  rankContainer: {
    width: 56,
    alignItems: 'center',
    justifyContent: 'center',
  },
  rank: {
    fontSize: 16,
    fontWeight: '600',
    color: '#666',
  },
  topRank: {
    fontSize: 18,
    fontWeight: '700',
    color: '#1a73e8',
  },
  userContainer: {
    flex: 1,
    paddingHorizontal: 12,
  },
  username: {
    fontSize: 16,
    fontWeight: '500',
    color: '#333',
  },
  topUsername: {
    fontSize: 17,
    fontWeight: '700',
    color: '#1a73e8',
  },
  ratingContainer: {
    width: 80,
    alignItems: 'flex-end',
  },
  rating: {
    fontSize: 16,
    fontWeight: '600',
    color: '#4caf50',
  },
  topRating: {
    fontSize: 17,
    fontWeight: '700',
    color: '#2e7d32',
  },
});

export default LeaderboardItem;
