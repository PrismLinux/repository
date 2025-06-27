<script setup lang="ts">
import { ref, onMounted, computed, watch } from "vue";

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
const searchTerm = ref("");
const archFilter = ref("");
const sortFilter = ref("name"); // 'name', 'date', 'size'
const loading = ref(true); // Keep true initially to show loading state
const error = ref<string | null>(null);

async function loadPackages() {
  console.log("loadPackages: Starting package load process.");
  loading.value = true;
  error.value = null;
  try {
    const fetchUrl = `./api/packages.json?v=${new Date().getTime()}`;
    console.log(`loadPackages: Attempting to fetch from ${fetchUrl}`);
    const response = await fetch(fetchUrl);

    if (!response.ok) {
      console.error(`loadPackages: HTTP error! Status: ${response.status}`);
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    console.log("loadPackages: Fetch successful, attempting to parse JSON.");
    const data = await response.json();

    if (!Array.isArray(data)) {
      console.error("loadPackages: Loaded data is not an array:", data);
      allPackages.value = [];
      error.value = "Invalid package data format.";
    } else {
      console.log(`loadPackages: Successfully loaded ${data.length} packages.`);
      allPackages.value = data;
    }
  } catch (err: any) {
    console.error("loadPackages: Error loading or parsing packages:", err);
    error.value = `Failed to load packages: ${err.message}. Please try again later.`;
  } finally {
    console.log(
      "loadPackages: Package load process finished. Setting loading to false.",
    );
    loading.value = false;
  }
}

const filteredAndSortedPackages = computed<Package[]>(() => {
  let tempPackages = [...allPackages.value];

  // Filter
  const lowerSearchTerm = searchTerm.value.toLowerCase();
  tempPackages = tempPackages.filter((pkg) => {
    const matchesSearch =
      (pkg.name?.toLowerCase() || "").includes(lowerSearchTerm) ||
      (pkg.description?.toLowerCase() || "").includes(lowerSearchTerm);
    const matchesArch =
      !archFilter.value || pkg.architecture === archFilter.value;
    return matchesSearch && matchesArch;
  });

  // Sort
  tempPackages.sort((a, b) => {
    switch (sortFilter.value) {
      case "name":
        return (a.name || "").localeCompare(b.name || "");
      case "date":
        return new Date(b.modified).getTime() - new Date(a.modified).getTime();
      case "size":
        const sizeToBytes = (sizeStr: string) => {
          if (!sizeStr) return 0;
          const size = parseFloat(sizeStr);
          if (sizeStr.endsWith("K")) return size * 1024;
          if (sizeStr.endsWith("M")) return size * 1024 * 1024;
          if (sizeStr.endsWith("G")) return size * 1024 * 1024 * 1024;
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
const shownPackagesCount = computed(
  () => filteredAndSortedPackages.value.length,
);

// Call loadPackages when the component is mounted
onMounted(() => {
  console.log("PackagesView: Component mounted. Calling loadPackages().");
  loadPackages();
});

// Watch for changes in filters and search term to re-filter/sort
watch([searchTerm, archFilter, sortFilter], () => {
  // Computed property already handles reactivity, no explicit re-render call needed
  console.log(
    "PackagesView: Filter or sort parameter changed. Re-computing package list.",
  );
});

// Expose loadPackages if parent App.vue needs to trigger a reload manually
defineExpose({
  loadPackages,
});
</script>

<template>
  <div>
    <div class="stats" id="stats-container">
      <div class="stat-item">
        <div class="stat-number" id="total-packages">
          {{ totalPackagesCount }}
        </div>
        <div class="stat-label">Total Packages</div>
      </div>
      <div class="stat-item">
        <div class="stat-number" id="filtered-packages">
          {{ shownPackagesCount }}
        </div>
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
      />
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
      <div
        v-else-if="filteredAndSortedPackages.length === 0"
        class="no-results"
      >
        No packages found matching your criteria.
      </div>
      <div v-else class="package-grid">
        <div
          v-for="pkg in filteredAndSortedPackages"
          :key="pkg.filename"
          class="package-card"
        >
          <div class="package-name">{{ pkg.name || "N/A" }}</div>
          <div class="package-version">Version: {{ pkg.version || "N/A" }}</div>
          <div class="package-desc">
            {{ pkg.description || "No description." }}
          </div>
          <div class="package-meta">
            <span
              ><strong>Architecture:</strong>
              {{ pkg.architecture || "N/A" }}</span
            >
            <span><strong>Size:</strong> {{ pkg.size || "N/A" }}</span>
            <span><strong>Modified:</strong> {{ pkg.modified || "N/A" }}</span>
          </div>
          <a :href="`./x86_64/${pkg.filename}`" class="download-btn" download
            >Download Package</a
          >
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Scoped styles for PackagesView.vue */
.stats {
  display: flex;
  justify-content: space-around;
  background: #f8f9fa;
  padding: 20px;
  border-radius: 8px;
  margin-bottom: 20px;
}
.stat-item {
  text-align: center;
}
.stat-number {
  font-size: 2em;
  font-weight: bold;
  color: #3498db;
}
.stat-label {
  color: #666;
  margin-top: 5px;
}
.search-container {
  margin-bottom: 20px;
}
.search-input {
  width: 100%;
  padding: 12px;
  font-size: 16px;
  border: 2px solid #ddd;
  border-radius: 6px;
  transition: border-color 0.3s;
}
.search-input:focus {
  outline: none;
  border-color: #3498db;
}
.filters {
  display: flex;
  gap: 15px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}
.filter-select {
  padding: 8px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
  background: white;
}
.package-grid {
  display: grid;
  gap: 20px;
  grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
}
.package-card {
  border: 1px solid #ddd;
  border-radius: 8px;
  padding: 20px;
  transition: all 0.3s;
  background: white;
}
.package-card:hover {
  box-shadow: 0 5px 15px rgba(0, 0, 0, 0.1);
  transform: translateY(-2px);
}
.package-name {
  font-size: 1.3em;
  font-weight: bold;
  color: #2c3e50;
  margin-bottom: 8px;
}
.package-version {
  color: #3498db;
  font-weight: 500;
  margin-bottom: 10px;
}
.package-desc {
  color: #666;
  margin-bottom: 15px;
  line-height: 1.4;
}
.package-meta {
  font-size: 0.9em;
  color: #888;
}
.package-meta span {
  display: inline-block;
  margin-right: 15px;
  margin-bottom: 5px;
}
.download-btn {
  background: #27ae60;
  color: white;
  padding: 8px 16px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  text-decoration: none;
  display: inline-block;
  margin-top: 10px;
  transition: background 0.3s;
}
.download-btn:hover {
  background: #219a52;
}
.loading,
.no-results {
  text-align: center;
  padding: 40px;
  color: #666;
  font-style: italic;
}
</style>
