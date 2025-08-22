<script setup lang="ts">
import { ref, watchEffect } from "vue";
import PackagesView from "./components/PackagesView.vue";

const activeTab = ref("packages");

// Update active tab based on URL hash for direct linking
watchEffect(() => {
  activeTab.value = "packages";

  // Page <<Setup>> disabled.
  /*
  const hash = window.location.hash.slice(1);
  if (hash === "packages") {
    activeTab.value = "packages";
  } else {
    activeTab.value = "setup";
  }
  */
});

function showTab(tabName: string) {
  activeTab.value = tabName;
  window.location.hash = tabName; // Update URL hash
}
</script>

<template>
  <div>
    <div class="header">
      <h1>PrismLinux Repository</h1>
      <p>
        Arch Linux compatible package repository for PrismLinux distribution
      </p>
    </div>

    <div class="nav-tabs">
      <!-- <button
        :class="['nav-tab', { active: activeTab === 'setup' }]"
        @click="showTab('setup')"
      >
        Setup
      </button> -->
      <button
        :class="['nav-tab', { active: activeTab === 'packages' }]"
        @click="showTab('packages')"
      >
        Browse Packages
      </button>
    </div>

    <!-- <div v-show="activeTab === 'setup'" class="tab-content">
      <SetupView />
    </div> -->

    <div v-show="activeTab === 'packages'" class="tab-content">
      <PackagesView />
    </div>
  </div>
</template>
