<template>
  <div>
    <h2>Repository Setup</h2>
    <div class="setup-section">
      <h3>Quick Setup</h3>
      <p>Add the following to your <code>/etc/pacman.conf</code>:</p>
      <div class="code">
        [crystallinux]<br />Server =
        https://crystalnetwork-studio.gitlab.io/linux/CrystalLinux/tooling/package-repository/$arch<br />SigLevel
        = Optional TrustAll
      </div>

      <p>Then run:</p>
      <div class="code">sudo pacman -Sy</div>

      <h3>Install Packages</h3>
      <p>Install packages from the repository:</p>
      <div class="code">sudo pacman -S package-name</div>

      <h3>Search Repository</h3>
      <p>Search for available packages:</p>
      <div class="code">pacman -Ss crystallinux/</div>
    </div>
  </div>
</template>
