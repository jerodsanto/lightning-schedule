// Auto-refresh when returning to standalone app (iOS home screen app)
if (window.matchMedia("(display-mode: standalone)").matches) {
  // Detect when page becomes visible (user returns to the app)
  document.addEventListener("visibilitychange", function () {
    if (!document.hidden) {
      window.location.reload();
    }
  });

  // Also handle iOS-specific pageshow event (detects app resume from background)
  window.addEventListener("pageshow", function (event) {
    if (event.persisted) {
      window.location.reload();
    }
  });
}

function handleTimestamps() {
  const lastUpdatedEl = document.getElementById("lastUpdated");
  if (lastUpdatedEl) {
    const utcTime = lastUpdatedEl.getAttribute("data-utc");
    if (utcTime) {
      try {
        const date = new Date(utcTime);
        // Format in Central Time (America/Chicago)
        const options = {
          timeZone: "America/Chicago",
          month: "numeric",
          day: "numeric",
          year: "2-digit",
          hour: "numeric",
          minute: "2-digit",
          hour12: true,
        };
        const formatter = new Intl.DateTimeFormat("en-US", options);
        const parts = formatter.formatToParts(date);

        const month = parts.find((p) => p.type === "month").value;
        const day = parts.find((p) => p.type === "day").value;
        const year = parts.find((p) => p.type === "year").value;
        const hour = parts.find((p) => p.type === "hour").value;
        const minute = parts.find((p) => p.type === "minute").value;
        const dayPeriod = parts.find((p) => p.type === "dayPeriod").value;

        lastUpdatedEl.textContent =
          month +
          "/" +
          day +
          "/" +
          year +
          " at " +
          hour +
          ":" +
          minute +
          dayPeriod;
      } catch (e) {
        // Keep the UTC fallback if conversion fails
      }
    }
  }
}

function applyFilters() {
  const onlyUpcomingEl = document.getElementById("onlyUpcoming");

  // Load saved preference from localStorage
  if (localStorage.getItem("onlyUpcoming") === "true") {
    onlyUpcomingEl.classList.add("active");
    hidePastGames();
  }

  // Add event listener for filter changes
  onlyUpcomingEl.addEventListener("click", function () {
    const isActive = this.classList.contains("active");

    if (isActive) {
      localStorage.setItem("onlyUpcoming", false);
      onlyUpcomingEl.classList.remove("active");
      showPastGames();
    } else {
      localStorage.setItem("onlyUpcoming", true);
      onlyUpcomingEl.classList.add("active");
      hidePastGames();
    }
  });

  function hidePastGames() {
    const pastGames = document.querySelectorAll("tr.past-game");
    pastGames.forEach(function (row) {
      row.style.display = "none";
    });
  }

  function showPastGames() {
    const pastGames = document.querySelectorAll("tr.past-game");
    pastGames.forEach(function (row) {
      row.style.display = "";
    });
  }
}

function throttle(fn, context) {
  let frameId;
  return function (...args) {
    const contextBoundFn = fn.bind(context);
    if (frameId) return;
    frameId = requestAnimationFrame(() => {
      contextBoundFn(...args);
      frameId = null;
    });
  };
}

function syncTableHeaders() {
  const headerTable = document.querySelector(".schedule-header table");
  const bodyContainer = document.querySelector(".schedule-body");
  const bodyTable = bodyContainer.querySelector("table");

  // Throttled sync for horizontal scroll (runs ~60 FPS max)
  const throttledSync = throttle(() => {
    headerTable.style.transform = `translateX(-${bodyContainer.scrollLeft}px)`;
  });

  bodyContainer.addEventListener("scroll", throttledSync);

  // Match column widths by reading from first visible row's cells
  const headerThs = headerTable.querySelectorAll("th");
  const firstGameRow = bodyTable.querySelector("tbody tr.game-row");
  if (firstGameRow) {
    const bodyCells = firstGameRow.querySelectorAll("td");
    headerThs.forEach((headerTh, i) => {
      if (bodyCells[i]) {
        // Get the computed padding from the td
        const tdStyles = window.getComputedStyle(bodyCells[i]);
        const tdPaddingLeft = parseFloat(tdStyles.paddingLeft);
        const tdPaddingRight = parseFloat(tdStyles.paddingRight);

        // Calculate content width (offsetWidth includes padding and border)
        const contentWidth =
          bodyCells[i].offsetWidth - tdPaddingLeft - tdPaddingRight;

        // Set the th width to content width (its padding will be added on top)
        headerTh.style.width = `${contentWidth}px`;
      }
    });
  }
}

document.addEventListener("DOMContentLoaded", applyFilters);
document.addEventListener("DOMContentLoaded", handleTimestamps);
document.addEventListener("DOMContentLoaded", syncTableHeaders);
window.addEventListener("resize", syncTableHeaders);
