(() => {
  let initial;
  const poll = async () => {
    try {
      const response = await fetch('/__dev/version', {cache: 'no-store'});
      if (!response.ok) return;
      const version = await response.text();
      if (initial === undefined) { initial = version; return; }
      if (version !== initial) window.location.reload();
    } catch (_) {}
  };
  poll();
  window.setInterval(poll, 900);
})();
