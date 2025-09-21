const ops = ["sum", "count", "avg"];
const refreshInterval = 30000;
const width = 800;
const height = 500;
document.addEventListener("DOMContentLoaded", () => loadMetrics(renderMetrics));

async function loadMetrics(cb) {
  const container = document.querySelector(".metrics");
  if (!container) return;

  try {
    const res = await fetch("/metric", {
      headers: { Accept: "application/json" },
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const metrics = await res.json();

    cb(container, metrics.metrics);
  } catch (err) {
    console.error("Failed to load metrics:", err);
    container.textContent = "Failed to load metrics.";
  }
}

function renderMetrics(container, metrics) {
  container.innerHTML = "";
  const frag = document.createDocumentFragment();

  for (const name of metrics) {
    for (const op of ops) {
      const wrapper = document.createElement("div");
      wrapper.className = "col-lg-4 col-md-6 col-sm-12";

      const img = document.createElement("img");
      img.loading = "lazy";
      img.alt = `${name} ${op}`;
      img.id = `${name}-${op}`;
      img.className = "image-responsive";
      img.src = `/metric/${encodeURIComponent(
        name
      )}/${op}.png?width=${width}&height=${height}`;

      wrapper.appendChild(img);
      frag.appendChild(wrapper);
    }
  }

  container.appendChild(frag);

  setInterval(() => refreshMetrics(container, metrics), refreshInterval);
}

function refreshMetrics(container, metrics) {
  const now = new Date().valueOf();
  for (const name of metrics) {
    for (const op of ops) {
      const img = document.getElementById(`${name}-${op}`);
      img.src = `/metric/${encodeURIComponent(
        name
      )}/${op}.png?t=${now}&width=${width}&height=${height}`;
    }
  }
}
