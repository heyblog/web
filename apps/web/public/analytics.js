(() => {
  const start = () => {
    window.dataLayer = window.dataLayer || [];
    window.gtag = function () {
      window.dataLayer.push(arguments);
    };
    window.gtag('js', new Date());
    window.gtag('config', 'G-PH5EGCPHXH');

    const cloudflare = document.createElement('script');
    cloudflare.async = true;
    cloudflare.src = 'https://static.cloudflareinsights.com/beacon.min.js';
    cloudflare.dataset.cfBeacon = JSON.stringify({
      token: 'd76b65b3f55b4b8f96a0ac0ddcd5493e',
      spa: false,
    });
    document.head.append(cloudflare);

    const google = document.createElement('script');
    google.async = true;
    google.src = 'https://www.googletagmanager.com/gtag/js?id=G-PH5EGCPHXH';
    document.head.append(google);
  };

  // Start third-party requests after the page's load event has finished.
  const schedule = () => window.setTimeout(start, 0);
  if (document.readyState === 'complete') {
    schedule();
  } else {
    window.addEventListener('load', schedule, { once: true });
  }
})();
