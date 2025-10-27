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

function syncTableHeaders() {
  const headerTable = document.querySelector(".schedule-header table");
  const bodyContainer = document.querySelector(".schedule-body");

  // Sync horizontal scroll
  bodyContainer.addEventListener("scroll", () => {
    headerTable.style.transform = `translateX(-${bodyContainer.scrollLeft}px)`;
  });

  // Match column widths
  const headerThs = headerTable.querySelectorAll("th");
  const bodyThs = document.querySelectorAll(".body-table th");
  headerThs.forEach((headerTh, i) => {
    if (bodyThs[i]) {
      headerTh.style.width = `${bodyThs[i].offsetWidth}px`;
    }
  });
}

document.addEventListener("DOMContentLoaded", applyFilters);
document.addEventListener("DOMContentLoaded", handleTimestamps);
document.addEventListener("DOMContentLoaded", syncTableHeaders);
window.addEventListener("resize", syncTableHeaders);
