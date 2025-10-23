// Auto-refresh when returning to standalone app (iOS home screen app)
if (window.matchMedia("(display-mode: standalone)").matches) {
  // Detect when page becomes visible (user returns to the app)
  document.addEventListener("visibilitychange", function () {
    if (!document.hidden) {
      // Page is now visible - reload to get fresh data
      window.location.reload();
    }
  });

  // Also handle iOS-specific pageshow event (detects app resume from background)
  window.addEventListener("pageshow", function (event) {
    if (event.persisted) {
      // Page was loaded from cache (user returned to app)
      window.location.reload();
    }
  });
}

// Convert UTC timestamp to Central Time
document.addEventListener("DOMContentLoaded", function () {
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

  // Handle tooltip clicks on mobile devices
  const locationWrappers = document.querySelectorAll(".location-wrapper");

  locationWrappers.forEach(function (wrapper) {
    wrapper.addEventListener("click", function (e) {
      e.stopPropagation();

      // Close all other open tooltips
      locationWrappers.forEach(function (otherWrapper) {
        if (otherWrapper !== wrapper) {
          otherWrapper.classList.remove("active");
        }
      });

      // Toggle this tooltip
      wrapper.classList.toggle("active");
    });
  });

  // Close tooltips when clicking outside
  document.addEventListener("click", function () {
    locationWrappers.forEach(function (wrapper) {
      wrapper.classList.remove("active");
    });
  });

  // Handle "Only show upcoming" filter
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
});
