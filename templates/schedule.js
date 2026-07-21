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
    const pastNotes = document.querySelectorAll("tr.past-note");
    pastNotes.forEach(function (row) {
      row.style.display = "none";
    });
  }

  function showPastGames() {
    const pastGames = document.querySelectorAll("tr.past-game");
    pastGames.forEach(function (row) {
      row.style.display = "";
    });
    const pastNotes = document.querySelectorAll("tr.past-note");
    pastNotes.forEach(function (row) {
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
  // Placeholder-only pages (no games or notes) render without the schedule tables
  if (!headerTable || !bodyContainer) return;
  const bodyTable = bodyContainer.querySelector("table");
  const headerThs = headerTable.querySelectorAll("th");

  function syncWidths() {
    // Any visible game row reflects the column widths; note rows span all
    // columns and hidden (filtered) rows have no geometry to measure
    let cells = null;
    for (const row of bodyTable.querySelectorAll("tbody tr.game-row")) {
      if (row.getClientRects().length) {
        cells = row.children;
        break;
      }
    }
    if (!cells) return;

    // Pin the header table to the body table's exact width, then copy each
    // column's border-box width (th uses box-sizing: border-box in CSS)
    headerTable.style.width = `${bodyTable.getBoundingClientRect().width}px`;
    headerThs.forEach((th, i) => {
      if (cells[i]) {
        th.style.width = `${cells[i].getBoundingClientRect().width}px`;
      }
    });
  }

  // Throttled sync for horizontal scroll (runs ~60 FPS max)
  bodyContainer.addEventListener(
    "scroll",
    throttle(() => {
      headerTable.style.transform = `translateX(-${bodyContainer.scrollLeft}px)`;
    }),
  );

  // The body table's size changes whenever its layout does (viewport resize,
  // filter toggles hiding rows, fonts loading), so observing it re-syncs all
  if (window.ResizeObserver) {
    new ResizeObserver(throttle(syncWidths)).observe(bodyTable);
  } else {
    window.addEventListener("resize", throttle(syncWidths));
  }
  syncWidths();
}

document.addEventListener("DOMContentLoaded", applyFilters);
document.addEventListener("DOMContentLoaded", handleTimestamps);
document.addEventListener("DOMContentLoaded", syncTableHeaders);
