document.addEventListener("DOMContentLoaded", () => loadMetrics());

async function loadMetrics() {
  const container = document.querySelector(".metrics");
  if (!container) return;

  try {
    const res = await fetch("/metric", {
      headers: { Accept: "application/json" },
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const metrics = await res.json();
    console.log({ metrics });

    renderMetrics(container, metrics.metrics);
  } catch (err) {
    console.error("Failed to load metrics:", err);
    container.textContent = "Failed to load metrics.";
  }
}

function renderMetrics(container, metrics) {
  const ops = ["sum", "count", "avg"];
  container.innerHTML = "";
  const frag = document.createDocumentFragment();

  for (const name of metrics) {
    for (const op of ops) {
      const wrapper = document.createElement("div");
      wrapper.className = "col-4 col-md-6 col-sm-12";

      const img = document.createElement("img");
      img.loading = "lazy";
      img.alt = `${name} ${op}`;
      img.src = `/metric/${encodeURIComponent(name)}/${op}.png`;

      wrapper.appendChild(img);
      frag.appendChild(wrapper);
    }
  }

  container.appendChild(frag);
}
