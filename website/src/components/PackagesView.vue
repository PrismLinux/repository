<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';

interface Package {
  name: string;
  version: string;
  description: string;
  architecture: string;
  filename: string;
  size: string;
  modified: string;
  depends: string;
  groups: string;
}

const allPackages = ref<Package[]>([]);
const searchTerm = ref('');
const archFilter = ref('');
const sortFilter = ref('name'); // 'name', 'date', 'size'
const loading = ref(true);
const error = ref<string | null>(null);

async function loadPackages() {
  loading.value = true;
  error.value = null;
  try {
    // Add a cache-busting parameter to ensure fresh data
    const response = await fetch(`./api/packages.json?v=${new Date().getTime()}`);
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    const data = await response.json();
    if (!Array.isArray(data)) {
      console.error("Loaded data is not an array:", data);
      allPackages.value = [];
      error.value = "Invalid package data format.";
    } else {
      allPackages.value = data;
    }
  } catch (err: any) {
    console.error("Error loading or parsing packages:", err);
    error.value = `Failed to load packages: ${err.message}. Please try again later.`;
  } finally {
    loading.value = false;
  }
}

const filteredAndSortedPackages = computed<Package[]>(() => {
  let tempPackages = [...allPackages.value];

  // Filter
  const lowerSearchTerm = searchTerm.value.toLowerCase();
  tempPackages = tempPackages.filter(pkg => {
    const matchesSearch = (pkg.name?.toLowerCase() || '').includes(lowerSearchTerm) ||
                          (pkg.description?.toLowerCase() || '').includes(lowerSearchTerm);
    const matchesArch = !archFilter.value || pkg.architecture === archFilter.value;
    return matchesSearch && matchesArch;
  });

  // Sort
  tempPackages.sort((a, b) => {
    switch (sortFilter.value) {
      case 'name':
        return (a.name || '').localeCompare(b.name || '');
      case 'date':
        return new Date(b.modified).getTime() - new Date(a.modified).getTime();
      case 'size':
        const sizeToBytes = (sizeStr: string) => {
          if (!sizeStr) return 0;
          const size = parseFloat(sizeStr);
          if (sizeStr.endsWith('K')) return size * 1024;
          if (sizeStr.endsWith('M')) return size * 1024 * 1024;
          if (sizeStr.endsWith('G')) return size * 1024 * 1024 * 1024;
          return size;
        };
        return sizeToBytes(b.size) - sizeToBytes(a.size);
      default:
        return 0;
    }
  });

  return tempPackages;
});

const totalPackagesCount = computed(() => allPackages.value.length);
const shownPackagesCount = computed(() => filteredAndSortedPackages.value.length);

onMounted(() => {
  // Load packages only when this component is mounted (i.e., when 'Browse Packages' tab is first clicked)
  // This is handled by the App.vue now. The direct loadPackages onMounted is useful if this component
  // were to be used standalone, but App.vue handles the initial load for 'packages' tab.
  // For robustness, we can keep it here, as it won't re-fetch if allPackages.value is already populated.
  if (allPackages.value.length === 0 && !loading.value && !error.value) {
    loadPackages();
  }
});

// Expose loadPackages to parent (App.vue) if needed for initial tab switch
defineExpose({
  loadPackages
});
</script>

<template>
  <div>
    <div class="stats" id="stats-container">
      <div class="stat-item">
        <div class="stat-number" id="total-packages">{{ totalPackagesCount }}</div>
        <div class="stat-label">Total Packages</div>
      </div>
      <div class="stat-item">
        <div class="stat-number" id="filtered-packages">{{ shownPackagesCount }}</div>
        <div class="stat-label">Shown</div>
      </div>
    </div>

    <div class="search-container">
      <input
        type="text"
        class="search-input"
        id="search-input"
        placeholder="Search packages by name or description..."
        v-model="searchTerm"
      >
    </div>

    <div class="filters">
      <select class="filter-select" id="arch-filter" v-model="archFilter">
        <option value="">All Architectures</option>
        <option value="x86_64">x86_64</option>
        <option value="any">any</option>
      </select>
      <select class="filter-select" id="sort-filter" v-model="sortFilter">
        <option value="name">Sort by Name</option>
        <option value="date">Sort by Date</option>
        <option value="size">Sort by Size</option>
      </select>
    </div>

    <div id="packages-container">
      <div v-if="loading" class="loading">Loading packages...</div>
      <div v-else-if="error" class="no-results">{{ error }}</div>
      <div v-else-if="filteredAndSortedPackages.length === 0" class="no-results">No packages found matching your criteria.</div>
      <div v-else class="package-grid">
        <div v-for="pkg in filteredAndSortedPackages" :key="pkg.filename" class="package-card">
          <div class="package-name">{{ pkg.name || 'N/A' }}</div>
          <div class="package-version">Version: {{ pkg.version || 'N/A' }}</div>
          <div class="package-desc">{{ pkg.description || 'No description.' }}</div>
          <div class="package-meta">
            <span><strong>Architecture:</strong> {{ pkg.architecture || 'N/A' }}</span>
            <span><strong>Size:</strong> {{ pkg.size || 'N/A' }}</span>
            <span><strong>Modified:</strong> {{ pkg.modified || 'N/A' }}</span>
          </div>
          <a :href="`./x86_64/${pkg.filename}`" class="download-btn" download>Download Package</a>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* No specific scoped styles needed here, using global styles from style.css */
</style>
