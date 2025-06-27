<script setup lang="ts">
import { ref } from "vue";

// Track copy status for 4 code blocks
const copiedStatus = ref<boolean[]>(Array(4).fill(false));

// Copy text to clipboard
const copyToClipboard = async (text: string, index: number) => {
  try {
    await navigator.clipboard.writeText(text);
    copiedStatus.value[index] = true;
    setTimeout(() => {
      copiedStatus.value[index] = false;
    }, 1500);
  } catch (err) {
    console.error("Failed to copy text:", err);
  }
};
</script>

<template>
  <div>
    <h2>Repository Setup</h2>
    <div class="setup-section">
      <h3>Quick Setup</h3>
      <p>For a quick setup, run the following command in your terminal:</p>
      <div class="code">
        <code>
          sudo bash -c "$(curl -fsSL
          https://gitlab.com/crystalnetwork-studio/linux/CrystalLinux/tooling/package-repository/-/raw/master/install.sh)"
        </code>
        <button
          class="copy-button"
          @click="
            copyToClipboard(
              'sudo bash -c &quot;$(curl -fsSL https://gitlab.com/crystalnetwork-studio/linux/CrystalLinux/tooling/package-repository/-/raw/master/install.sh)&quot;',
              0,
            )
          "
          aria-label="Copy command"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
            fill="currentColor"
            viewBox="0 0 16 16"
          >
            <path
              fill-rule="evenodd"
              d="M4 2a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2zm2-1a1 1 0 0 0-1 1v8a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V2a1 1 0 0 0-1-1zM2 5a1 1 0 0 0-1 1v8a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1v-1h1v1a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h1v1z"
            />
          </svg>
        </button>
        <span v-show="copiedStatus[0]" class="copied-feedback">Copied!</span>
      </div>

      <p>Then run:</p>
      <div class="code">
        <code>sudo pacman -Sy</code>
        <button
          class="copy-button"
          @click="copyToClipboard('sudo pacman -Sy', 1)"
          aria-label="Copy command"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
            fill="currentColor"
            viewBox="0 0 16 16"
          >
            <path
              fill-rule="evenodd"
              d="M4 2a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2zm2-1a1 1 0 0 0-1 1v8a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V2a1 1 0 0 0-1-1zM2 5a1 1 0 0 0-1 1v8a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1v-1h1v1a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h1v1z"
            />
          </svg>
        </button>
        <span v-show="copiedStatus[1]" class="copied-feedback">Copied!</span>
      </div>

      <h3>Install Packages</h3>
      <p>Install packages from the repository:</p>
      <div class="code">
        <code>sudo pacman -S package-name</code>
        <button
          class="copy-button"
          @click="copyToClipboard('sudo pacman -S package-name', 2)"
          aria-label="Copy command"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
            fill="currentColor"
            viewBox="0 0 16 16"
          >
            <path
              fill-rule="evenodd"
              d="M4 2a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2zm2-1a1 1 0 0 0-1 1v8a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V2a1 1 0 0 0-1-1zM2 5a1 1 0 0 0-1 1v8a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1v-1h1v1a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h1v1z"
            />
          </svg>
        </button>
        <span v-show="copiedStatus[2]" class="copied-feedback">Copied!</span>
      </div>

      <h3>Uninstall Repository</h3>
      <p>To uninstall the repository, run the following command:</p>
      <div class="code">
        <code>
          sudo bash -c "$(curl -fsSL
          https://gitlab.com/crystalnetwork-studio/linux/CrystalLinux/tooling/package-repository/-/raw/master/install.sh)"
          -- --uninstall
        </code>
        <button
          class="copy-button"
          @click="
            copyToClipboard(
              'sudo bash -c &quot;$(curl -fsSL https://gitlab.com/crystalnetwork-studio/linux/CrystalLinux/tooling/package-repository/-/raw/master/install.sh)&quot; -- --uninstall',
              4,
            )
          "
          aria-label="Copy uninstall command"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
            fill="currentColor"
            viewBox="0 0 16 16"
          >
            <path
              fill-rule="evenodd"
              d="M4 2a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2zm2-1a1 1 0 0 0-1 1v8a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V2a1 1 0 0 0-1-1zM2 5a1 1 0 0 0-1 1v8a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1v-1h1v1a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h1v1z"
            />
          </svg>
        </button>
        <span v-show="copiedStatus[4]" class="copied-feedback">Copied!</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.code {
  background: #2c3e50;
  color: #ecf0f1;
  padding: 15px;
  padding-right: 50px;
  border-radius: 5px;
  font-family: "Courier New", monospace;
  overflow: visible;
  margin: 10px 0;
  position: relative;
  display: block;
}

.code code {
  display: block;
}

.copy-button {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 0.9em;
  transition: all 0.2s ease;
  flex-shrink: 0;
  position: absolute;
  right: 10px;
  top: 50%;
  transform: translateY(-50%);
  padding: 5px;
  display: flex;
  justify-content: center;
  align-items: center;
  color: #ecf0f1;
  opacity: 0.7;
}

.copy-button:hover {
  background: rgba(255, 255, 255, 0.1);
  border-radius: 3px;
  opacity: 1;
}

.copy-button svg {
  fill: currentColor;
}

.copied-feedback {
  position: absolute;
  top: -27px;
  right: 0px;
  background-color: #27ae60;
  color: white;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.8em;
  z-index: 10;
  animation: fade-in-out 1.5s forwards;
  pointer-events: none;
  white-space: nowrap;
}

@keyframes fade-in-out {
  0% {
    opacity: 0;
    transform: translateY(10px);
  }
  20% {
    opacity: 1;
    transform: translateY(0);
  }
  90% {
    opacity: 1;
    transform: translateY(0);
  }
  100% {
    opacity: 0;
    transform: translateY(-10px);
  }
}
</style>
