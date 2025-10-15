#!/usr/bin/env node
/**
 * Basketball Team Schedule Scraper
 * Scrapes multiple team schedules from tourneymachine.com and combines them into a single HTML page.
 */

const axios = require("axios");
const cheerio = require("cheerio");
const fs = require("fs").promises;
const path = require("path");

// Google Sheet ID for additional games
const GOOGLE_SHEET_ID = "1JG0KliyzTT8muoDPAhTJWBilE1iUQMm22XOq1H4N6aQ";
const GOOGLE_SHEET_CSV_URL = `https://docs.google.com/spreadsheets/d/${GOOGLE_SHEET_ID}/export?format=csv`;

// Location abbreviations
// Maps base location names (without court/gym) to shorter versions
const LOCATION_ABBREVIATIONS = {
  "UBT South Sports Complex (Attack-Elite)": "UBT South",
  "Trinity Classical Academy": "TCA",
  "Elkhorn North Ridge Middle School": "ENRMS",
  "Elkhorn Valley View Middle School": "Valley View",
  "Elkhorn Ridge Middle School": "ERMS",
  "Elkhorn Middle School": "Elkhorn Middle",
  "Elkhorn Grandview Middle School": "Grandview Middle",
  "EPS Woodbrook Elementary": "Woodbrook",
  "EPS West Dodge Station Elementary": "West Dodge Station",
  "EPS Arbor View Elementary": "Arbor View",
  "Nebraska Basketball Academy": "Nebraska Basketball Academy",
  "Iowa West Fieldhouse": "",
};

// Team URLs - Add more teams here
// Format: displayName: { url: "...", htmlName: "exact name as it appears in the HTML", color: "#RRGGBB" }
const TEAM_URLS = {
  Varsity: {
    url: null, // No URL - only from Google Sheet
    htmlName: null,
    color: "#f59c44", // orange
  },
  JV: {
    url: null, // No URL - only from Google Sheet
    htmlName: null,
    color: "#44a15b", // green
  },
  "14U Gold": {
    url: "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263785b6ed3896640&IDTeam=h2025080322162058474d91e7d042e47",
    htmlName: "Omaha Lightning Gold 8th",
    color: "#FFD700",
  },
  "14U White": {
    url: "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263785b6ed3896640&IDTeam=h20250803221620558cb62c45d697d46",
    htmlName: "Omaha Lightning White 8th",
    color: "#FFFFFF",
  },
  "12U Blue": {
    url: "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263029c941335204c&IDTeam=h20250803221620486ddba884e17c748",
    htmlName: "Omaha Lightning Blue 6th",
    color: "#5b9de9",
  },
  "10U Red": {
    url: "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263e6b6d69f385c49&IDTeam=h202508032216206132b484a6720f345",
    htmlName: "Omaha Lightning Red 4th",
    color: "#d53a44",
  },
  "10U Black": {
    url: "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263934d14719c5d45&IDTeam=h202508032216205157e930ef2d5314d",
    htmlName: "Omaha Lightning Black 3rd",
    color: "#000000",
  },
};

/**
 * Default team color for teams not in TEAM_URLS
 */
const DEFAULT_TEAM_COLOR = "#2196F3";

/**
 * Get team color from TEAM_URLS or return default
 */
function getTeamColor(teamName) {
  const teamEntry = Object.entries(TEAM_URLS).find(
    ([displayName]) => displayName === teamName,
  );
  return teamEntry ? teamEntry[1].color : DEFAULT_TEAM_COLOR;
}

/**
 * Get text color for a team badge based on the background color
 * Returns white for dark backgrounds, black for light backgrounds
 * Uses relative luminance calculation (WCAG formula)
 */
function getTeamTextColor(backgroundColor) {
  // Normalize the color to lowercase for comparison
  const normalizedColor = backgroundColor.toLowerCase();

  // Parse the color to RGB values
  let r, g, b;

  // Handle hex colors (#RRGGBB or #RGB)
  if (normalizedColor.startsWith("#")) {
    const hex = normalizedColor.substring(1);
    if (hex.length === 6) {
      r = parseInt(hex.substring(0, 2), 16);
      g = parseInt(hex.substring(2, 4), 16);
      b = parseInt(hex.substring(4, 6), 16);
    } else if (hex.length === 3) {
      r = parseInt(hex[0] + hex[0], 16);
      g = parseInt(hex[1] + hex[1], 16);
      b = parseInt(hex[2] + hex[2], 16);
    }
  }

  // Calculate relative luminance using WCAG formula
  // https://www.w3.org/TR/WCAG20/#relativeluminancedef
  const rsRGB = r / 255;
  const gsRGB = g / 255;
  const bsRGB = b / 255;

  const rLinear =
    rsRGB <= 0.03928 ? rsRGB / 12.92 : Math.pow((rsRGB + 0.055) / 1.055, 2.4);
  const gLinear =
    gsRGB <= 0.03928 ? gsRGB / 12.92 : Math.pow((gsRGB + 0.055) / 1.055, 2.4);
  const bLinear =
    bsRGB <= 0.03928 ? bsRGB / 12.92 : Math.pow((bsRGB + 0.055) / 1.055, 2.4);

  const luminance = 0.2126 * rLinear + 0.7152 * gLinear + 0.0722 * bLinear;

  // Use white text for dark colors (luminance < 0.5), black text for light colors
  return luminance < 0.5 ? "white" : "black";
}

/**
 * Parse CSV data from Google Sheets
 */
async function fetchGoogleSheetGames() {
  console.log("Fetching additional games from Google Sheet...");

  try {
    const response = await axios.get(GOOGLE_SHEET_CSV_URL, { timeout: 10000 });
    const csvData = response.data;
    const lines = csvData.split("\n");
    const games = [];

    // Skip header row (index 0) and parse data rows
    for (let i = 1; i < lines.length; i++) {
      const line = lines[i].trim();
      if (!line) continue;

      // Parse CSV line - handle quoted fields
      const fields = [];
      let currentField = "";
      let inQuotes = false;

      for (let j = 0; j < line.length; j++) {
        const char = line[j];

        if (char === '"') {
          inQuotes = !inQuotes;
        } else if (char === "," && !inQuotes) {
          fields.push(currentField.trim());
          currentField = "";
        } else {
          currentField += char;
        }
      }
      fields.push(currentField.trim()); // Add last field

      // Expected columns: Team, Date, Time, Location, Jersey, Opponent, Score
      if (fields.length >= 6) {
        const [team, date, time, location, jersey, opponent, score = ""] =
          fields;

        // Skip rows with missing critical data
        if (!team || !date || !opponent) continue;

        // Determine home/away from jersey field
        let homeAway = "";
        if (
          jersey.toLowerCase().includes("home") ||
          jersey.toLowerCase().includes("light")
        ) {
          homeAway = "Home";
        } else if (
          jersey.toLowerCase().includes("away") ||
          jersey.toLowerCase().includes("dark")
        ) {
          homeAway = "Away";
        }

        // Parse date to standard format (expecting MM/DD/YYYY or similar)
        let formattedDate = date;
        try {
          const dateObj = new Date(date);
          if (!isNaN(dateObj.getTime())) {
            formattedDate = dateObj.toLocaleDateString("en-US", {
              weekday: "long",
              month: "long",
              day: "numeric",
              year: "numeric",
            });
          }
        } catch (e) {
          // Keep original date if parsing fails
        }

        games.push({
          team: team,
          date: formattedDate,
          time: time || "TBD",
          location: location || "TBD",
          opponent: opponent,
          homeAway: homeAway,
          score: score || "-",
          color: getTeamColor(team),
        });
      }
    }

    console.log(`Found ${games.length} games in Google Sheet`);
    return games;
  } catch (error) {
    console.error("Error fetching Google Sheet:", error.message);
    return [];
  }
}

/**
 * Scrape schedule data for a single team
 */
async function scrapeTeamSchedule(displayName, url, htmlName, color) {
  console.log(`Scraping ${displayName}...`);

  try {
    const response = await axios.get(url, { timeout: 10000 });
    const $ = cheerio.load(response.data);
    const games = [];

    // Find all tables and look for schedule data
    $("table").each((_, table) => {
      const $table = $(table);
      let currentDate = "";

      // Look for all rows (both headers and data rows)
      $table.find("tr").each((_, row) => {
        const $row = $(row);

        // Check if this is a header row with a date
        const thCells = $row.find("th");
        if (thCells.length > 0) {
          const headerText = $row.text().trim();
          // Look for date pattern like "Saturday, October 18, 2025"
          if (headerText.match(/\w+day,\s+\w+\s+\d+,\s+\d{4}/)) {
            currentDate = headerText;
          }
        }

        // Look for table data rows
        const cells = $row.find("td");

        // The schedule table has 8 columns: Game, Time, Location, Visitor, Visitor Score, Home Score, Home, (blank)
        if (cells.length === 8 && currentDate) {
          const gameNum = $(cells[0]).text().trim();
          let time = $(cells[1]).text().trim();
          const location = $(cells[2]).text().trim();
          const visitor = $(cells[3]).text().trim();
          const visitorScore = $(cells[4]).text().trim();
          const homeScore = $(cells[5]).text().trim();
          const home = $(cells[6]).text().trim();

          // Remove date prefix from time if present (e.g., "Sat 10/18/25 6:00 PM" -> "6:00 PM")
          time = time.replace(
            /^(Mon|Tue|Wed|Thu|Fri|Sat|Sun)\s+\d+\/\d+\/\d+\s+/,
            "",
          );

          // Check if this row has valid time data (to distinguish from other 8-column rows)
          if (time && time.match(/\d+:\d+/)) {
            // Determine opponent based on whether our team is home or away
            let opponent = "";
            let homeAway = "";
            let score = "";

            if (visitor === htmlName) {
              opponent = home;
              homeAway = "Away";
              // Format score as "visitor-home" or use × if not played
              if (visitorScore !== "×" && homeScore !== "×") {
                score = `${visitorScore}-${homeScore}`;
              } else {
                score = "-";
              }
            } else if (home === htmlName) {
              opponent = visitor;
              homeAway = "Home";
              // Format score as "home-visitor" or use × if not played
              if (visitorScore !== "×" && homeScore !== "×") {
                score = `${homeScore}-${visitorScore}`;
              } else {
                score = "-";
              }
            } else {
              // Skip this row if it doesn't contain our team
              return;
            }

            games.push({
              team: displayName,
              date: currentDate,
              time: time,
              location: location,
              opponent: opponent,
              homeAway: homeAway,
              score: score,
              color: color,
            });
          }
        }
      });
    });

    console.log(`Found ${games.length} games for ${displayName}`);
    return games;
  } catch (error) {
    console.error(`Error fetching ${displayName}:`, error.message);
    return [];
  }
}

/**
 * Parse date string for sorting
 */
function parseDateForSorting(dateStr) {
  try {
    // Handle format like "Saturday, October 18, 2025"
    const date = new Date(dateStr);
    if (!isNaN(date.getTime())) {
      return date;
    }

    // If parsing fails, return far future date
    return new Date(2099, 11, 31);
  } catch (error) {
    return new Date(2099, 11, 31);
  }
}

/**
 * Parse time string to minutes for sorting
 */
function parseTimeToMinutes(timeStr) {
  try {
    // Parse time like "6:00 PM" or "10:30 AM"
    const match = timeStr.match(/(\d+):(\d+)\s*(AM|PM)/i);
    if (match) {
      let hours = parseInt(match[1]);
      const minutes = parseInt(match[2]);
      const ampm = match[3].toUpperCase();

      if (ampm === "PM" && hours !== 12) {
        hours += 12;
      } else if (ampm === "AM" && hours === 12) {
        hours = 0;
      }

      return hours * 60 + minutes;
    }
    return 9999; // Default to end of day if can't parse
  } catch (error) {
    return 9999;
  }
}

/**
 * Format time to remove unnecessary :00
 * Examples: "4:00 PM" -> "4PM", "9:30 AM" -> "9:30AM"
 */
function formatTime(timeStr) {
  if (!timeStr || timeStr === "TBD") return timeStr;

  // Match time pattern like "4:00 PM" or "9:30 AM"
  const match = timeStr.match(/(\d+):(\d+)\s*(AM|PM)/i);
  if (match) {
    const hours = match[1];
    const minutes = match[2];
    const ampm = match[3].toUpperCase();

    // If minutes are 00, omit them
    if (minutes === "00") {
      return `${hours}${ampm}`;
    } else {
      return `${hours}:${minutes}${ampm}`;
    }
  }

  return timeStr;
}

/**
 * Get abbreviated location with full name for tooltip
 * Separates court/gym info and displays in parentheses
 * Returns object with abbr, courtGym, and tooltipText properties
 * If abbreviation is blank/empty, returns null for abbr to indicate no tooltip should be shown
 *
 * Example: "Elkhorn North Ridge Middle School - Auxiliary Gym"
 *   -> abbr: "ENRMS", courtGym: "aux gym", tooltipText: "Elkhorn North Ridge Middle School"
 * Example with blank abbreviation:
 *   -> abbr: null, courtGym: null, tooltipText: "Full Location - Court Info"
 */
function getLocationDisplay(location) {
  if (!location || location === "TBD") {
    return { abbr: "TBD", courtGym: null, tooltipText: "TBD" };
  }

  // Split on hyphen to separate main location from court/gym info
  const parts = location.split(" - ");

  if (parts.length === 2) {
    const baseLocation = parts[0].trim();
    const courtGymInfo = parts[1].trim().toLowerCase();

    // Check if abbreviation exists in the map
    if (baseLocation in LOCATION_ABBREVIATIONS) {
      const abbreviated = LOCATION_ABBREVIATIONS[baseLocation];

      // If abbreviation is blank/empty, show full location without tooltip
      if (!abbreviated || abbreviated.trim() === "") {
        return {
          abbr: null,
          courtGym: null,
          tooltipText: location,
        };
      }

      // Return abbreviated location with separate court/gym info
      // Tooltip only shows the base location (not court/gym)
      return {
        abbr: abbreviated,
        courtGym: courtGymInfo,
        tooltipText: baseLocation,
      };
    }

    // No abbreviation found in map, but still format with court/gym in parentheses
    return {
      abbr: baseLocation,
      courtGym: courtGymInfo,
      tooltipText: baseLocation,
    };
  }

  // No hyphen separator - check if there's an abbreviation for the whole thing
  if (location in LOCATION_ABBREVIATIONS) {
    const abbreviated = LOCATION_ABBREVIATIONS[location];

    // If abbreviation is blank/empty, show full location without tooltip
    if (!abbreviated || abbreviated.trim() === "") {
      return { abbr: null, courtGym: null, tooltipText: location };
    }

    return { abbr: abbreviated, courtGym: null, tooltipText: location };
  }

  // If no abbreviation exists, use the full name for both
  return { abbr: location, courtGym: null, tooltipText: location };
}

/**
 * Generate HTML schedule page
 */
async function generateHtml(
  allGames,
  outputFile = "index.html",
  filterTeam = null,
) {
  // Filter games if a specific team is requested
  const gamesToDisplay = filterTeam
    ? allGames.filter((game) => game.team === filterTeam)
    : allGames;

  // Sort games by date and time
  const sortedGames = [...gamesToDisplay].sort((a, b) => {
    const dateA = parseDateForSorting(a.date);
    const dateB = parseDateForSorting(b.date);

    // First sort by date
    if (dateA.getTime() !== dateB.getTime()) {
      return dateA - dateB;
    }

    // If dates are the same, sort by time
    return parseTimeToMinutes(a.time) - parseTimeToMinutes(b.time);
  });

  // Get unique teams in the order they appear in TEAM_URLS, then alphabetically for others
  const teamOrder = Object.keys(TEAM_URLS);
  const teams = [...new Set(allGames.map((game) => game.team))].sort((a, b) => {
    const aIndex = teamOrder.indexOf(a);
    const bIndex = teamOrder.indexOf(b);

    // Both teams are in TEAM_URLS - use their configured order
    if (aIndex !== -1 && bIndex !== -1) {
      return aIndex - bIndex;
    }

    // Only a is in TEAM_URLS - it comes first
    if (aIndex !== -1) return -1;

    // Only b is in TEAM_URLS - it comes first
    if (bIndex !== -1) return 1;

    // Neither is in TEAM_URLS - sort alphabetically
    return a.localeCompare(b);
  });

  // Create a map of team names to colors
  const teamColorMap = {};
  allGames.forEach((game) => {
    teamColorMap[game.team] = game.color;
  });

  let html = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Lightning Game Schedule</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
            background-color: #f5f5f5;
        }
        h1 {
            color: #333;
            text-align: center;
        }
        .filter-buttons {
            text-align: center;
            margin: 20px 0;
        }
        .filter-btn {
            padding: 8px 16px;
            margin: 4px;
            border: none;
            cursor: pointer;
            border-radius: 4px;
            background-color: #999 !important;
            color: white !important;
            transition: all 0.2s;
        }
        .filter-btn:hover {
            background-color: #777 !important;
        }
        .filter-btn.active {
            /* Active styles set inline */
        }
        table {
            width: 100%;
            max-width: 1200px;
            margin: 0 auto;
            border-collapse: collapse;
            background-color: white;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        th {
            background-color: #fbcb44;
            color: black;
            padding: 12px;
            text-align: left;
        }
        td {
            padding: 10px;
            border-bottom: 1px solid #ddd;
        }
        tr.month-end td {
            border-bottom: 3px solid #fbcb44;
        }
        tr:hover {
            background-color: #f5f5f5;
        }
        .team-badge {
            display: inline-block;
            padding: 4px 8px;
            background-color: #2196F3;
            color: black;
            border-radius: 4px;
            font-size: 0.9em;
        }
        .home-away-badge {
            display: inline-block;
            padding: 2px 6px;
            border-radius: 3px;
            font-size: 0.8em;
            font-weight: bold;
            margin-left: 4px;
        }
        .home {
            background-color: #4CAF50;
            color: white;
        }
        .away {
            background-color: #FF9800;
            color: white;
        }
        .location-wrapper {
            position: relative;
            display: inline-block;
            cursor: help;
        }
        .location-abbr {
            text-decoration: underline;
            text-decoration-style: dotted;
            text-decoration-color: #999;
        }
        .location-tooltip {
            visibility: hidden;
            opacity: 0;
            position: absolute;
            z-index: 1000;
            background-color: #333;
            color: white;
            padding: 8px 12px;
            border-radius: 6px;
            font-size: 0.9em;
            white-space: nowrap;
            bottom: 125%;
            left: 50%;
            transform: translateX(-50%);
            transition: opacity 0.3s;
            box-shadow: 0 4px 6px rgba(0,0,0,0.3);
        }
        .location-tooltip::after {
            content: "";
            position: absolute;
            top: 100%;
            left: 50%;
            margin-left: -5px;
            border-width: 5px;
            border-style: solid;
            border-color: #333 transparent transparent transparent;
        }
        .location-wrapper:hover .location-tooltip {
            visibility: visible;
            opacity: 1;
        }
        .location-wrapper.active .location-tooltip {
            visibility: visible;
            opacity: 1;
        }
        @media (max-width: 768px) {
            body {
                margin: 10px;
            }
            h1 {
                font-size: 1.5em;
            }
            .filter-buttons {
                margin: 15px 0;
            }
            .filter-btn {
                padding: 6px 10px;
                font-size: 0.85em;
                margin: 2px;
            }
            table {
                font-size: 0.7em;
                display: block;
                overflow-x: auto;
                -webkit-overflow-scrolling: touch;
                white-space: nowrap;
            }
            thead, tbody {
                display: table;
                width: 100%;
            }
            th, td {
                padding: 8px 4px;
                word-wrap: break-word;
                white-space: normal;
            }
            /* Make team column narrower */
            th:nth-child(1),
            td:nth-child(1) {
                min-width: 60px;
            }
            /* Make date column narrower */
            th:nth-child(2),
            td:nth-child(2) {
                min-width: 85px;
            }
            /* Make time column narrower */
            th:nth-child(3),
            td:nth-child(3) {
                min-width: 50px;
            }
            /* Location needs more space */
            th:nth-child(4),
            td:nth-child(4) {
                min-width: 120px;
                max-width: 180px;
            }
            /* Jersey column */
            th:nth-child(5),
            td:nth-child(5) {
                min-width: 65px;
            }
            /* Opponent needs space */
            th:nth-child(6),
            td:nth-child(6) {
                min-width: 100px;
                max-width: 150px;
            }
            /* Score column */
            th:nth-child(7),
            td:nth-child(7) {
                min-width: 40px;
            }
            .team-badge {
                font-size: 0.85em;
                padding: 3px 5px;
            }
        }
        @media (max-width: 480px) {
            body {
                margin: 5px;
            }
            table {
                font-size: 0.65em;
            }
            th, td {
                padding: 6px 3px;
            }
            .team-badge {
                font-size: 0.8em;
                padding: 2px 4px;
            }
            .filter-btn {
                padding: 5px 8px;
                font-size: 0.8em;
            }
        }
    </style>
</head>
<body>
    <h1>⚡️ Lightning Game Schedule</h1>
    <p style="text-align: center; color: #999; font-size: 0.75rem; margin: -10px 0 20px 0;">Last updated on ${new Date().toLocaleDateString("en-US", { month: "numeric", day: "numeric", year: "2-digit" })} at ${new Date().toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit", hour12: true })}</p>
    <div class="filter-buttons">
        <a href="${filterTeam ? "../" : "./"}" class="filter-btn ${!filterTeam ? "active" : ""}" style="${!filterTeam ? "background-color: #fbcb44 !important; color: black !important;" : ""} text-decoration: none; display: inline-block;">All Teams</a>
`;

  // Add filter buttons for each team with their respective colors
  teams.forEach((team) => {
    const teamColor = teamColorMap[team];
    const textColor = getTeamTextColor(teamColor);
    const borderStyle =
      teamColor === "#FFFFFF" ? " border: 1px solid black;" : "";
    const teamSlug = team.toLowerCase().replace(/\s+/g, "");
    const activeClass = filterTeam === team ? "active" : "";
    const activeStyle =
      filterTeam === team
        ? `background-color: ${teamColor} !important; color: ${textColor} !important;${borderStyle}`
        : "";
    const teamLink = filterTeam ? `../${teamSlug}/` : `${teamSlug}/`;
    html += `        <a href="${teamLink}" class="filter-btn ${activeClass}" style="${activeStyle} text-decoration: none; display: inline-block;">${team}</a>\n`;
  });

  html += `    </div>
    <table id="scheduleTable">
        <thead>
            <tr>
                <th>Team</th>
                <th>Date</th>
                <th>Time</th>
                <th>Location</th>
                <th>Jersey</th>
                <th>Opponent</th>
                <th>Score</th>
            </tr>
        </thead>
        <tbody>
`;

  // Add game rows
  sortedGames.forEach((game, index) => {
    // Determine if this is the last game of its month
    let isMonthEnd = false;
    if (index < sortedGames.length - 1) {
      const currentDate = parseDateForSorting(game.date);
      const nextDate = parseDateForSorting(sortedGames[index + 1].date);

      // Check if month changes between this game and the next
      if (currentDate.getMonth() !== nextDate.getMonth() ||
          currentDate.getFullYear() !== nextDate.getFullYear()) {
        isMonthEnd = true;
      }
    } else {
      // Last game in the list is always a month end
      isMonthEnd = true;
    }
    // Format date to be more concise (e.g., "Sat, 10/18/25")
    let displayDate = game.date || "TBD";
    try {
      const dateObj = new Date(game.date);
      if (!isNaN(dateObj.getTime())) {
        const weekday = dateObj.toLocaleDateString("en-US", {
          weekday: "short",
        });
        const month = dateObj.getMonth() + 1; // 0-indexed
        const day = dateObj.getDate();
        const year = dateObj.getFullYear().toString().slice(-2); // Last 2 digits
        displayDate = `${weekday}, ${month}/${day}/${year}`;
      }
    } catch (e) {
      // Keep original date if parsing fails
    }

    // Format jersey text
    let jerseyText = "TBD";
    if (game.homeAway === "Home") {
      jerseyText = "⬜️";
    } else if (game.homeAway === "Away") {
      jerseyText = "⬛️";
    }

    // Get team color and text color
    const teamColor = game.color;
    const textColor = getTeamTextColor(teamColor);
    const borderStyle =
      teamColor === "#FFFFFF" ? " border: 1px solid black;" : "";

    // Format time
    const displayTime = formatTime(game.time || "TBD");

    // Get location display
    const locationDisplay = getLocationDisplay(game.location || "TBD");
    let locationHtml;

    if (locationDisplay.abbr === null) {
      // No abbreviation - show full location without tooltip
      locationHtml = locationDisplay.tooltipText;
    } else if (locationDisplay.courtGym) {
      // Has abbreviation and court/gym info - wrap only abbreviation with tooltip
      locationHtml = `<span class="location-wrapper"><span class="location-abbr">${locationDisplay.abbr}</span><span class="location-tooltip">${locationDisplay.tooltipText}</span></span> (${locationDisplay.courtGym})`;
    } else if (locationDisplay.abbr !== locationDisplay.tooltipText) {
      // Has abbreviation but no court/gym - wrap with tooltip
      locationHtml = `<span class="location-wrapper"><span class="location-abbr">${locationDisplay.abbr}</span><span class="location-tooltip">${locationDisplay.tooltipText}</span></span>`;
    } else {
      // Same value for both - no tooltip needed
      locationHtml = locationDisplay.abbr;
    }

    const monthEndClass = isMonthEnd ? " month-end" : "";
    html += `            <tr class="game-row${monthEndClass}" data-team="${game.team}">
                <td><span class="team-badge" style="background-color: ${teamColor}; color: ${textColor};${borderStyle}">${game.team}</span></td>
                <td>${displayDate}</td>
                <td>${displayTime}</td>
                <td>${locationHtml}</td>
                <td>${jerseyText}</td>
                <td>${game.opponent || "TBD"}</td>
                <td>${game.score || "-"}</td>
            </tr>
`;
  });

  html += `        </tbody>
    </table>
    <script>
        // Handle tooltip clicks on mobile devices
        document.addEventListener('DOMContentLoaded', function() {
            const locationWrappers = document.querySelectorAll('.location-wrapper');

            locationWrappers.forEach(function(wrapper) {
                wrapper.addEventListener('click', function(e) {
                    e.stopPropagation();

                    // Close all other open tooltips
                    locationWrappers.forEach(function(otherWrapper) {
                        if (otherWrapper !== wrapper) {
                            otherWrapper.classList.remove('active');
                        }
                    });

                    // Toggle this tooltip
                    wrapper.classList.toggle('active');
                });
            });

            // Close tooltips when clicking outside
            document.addEventListener('click', function() {
                locationWrappers.forEach(function(wrapper) {
                    wrapper.classList.remove('active');
                });
            });
        });
    </script>
</body>
</html>
`;

  await fs.writeFile(outputFile, html, "utf-8");
  console.log(`Generated ${outputFile}`);
}

/**
 * Main function to scrape all teams and generate combined schedule
 */
async function main() {
  console.log("Starting schedule scraper...\n");

  const allGames = [];

  // Fetch games from team URLs (skip teams without URLs)
  for (const [displayName, teamInfo] of Object.entries(TEAM_URLS)) {
    if (teamInfo.url) {
      const games = await scrapeTeamSchedule(
        displayName,
        teamInfo.url,
        teamInfo.htmlName,
        teamInfo.color,
      );
      allGames.push(...games);
    }
  }

  // Fetch additional games from Google Sheet
  const sheetGames = await fetchGoogleSheetGames();
  allGames.push(...sheetGames);

  if (allGames.length === 0) {
    console.log("\nNo games found. Please check the URLs and try again.");
    process.exit(1);
  }

  console.log(`\nTotal games found: ${allGames.length}`);

  // Create dist directory
  const distDir = path.join(process.cwd(), "dist");
  await fs.mkdir(distDir, { recursive: true });

  // Generate combined schedule as index.html in dist
  await generateHtml(allGames, path.join(distDir, "index.html"));

  // Generate individual team schedules in subfolders
  const teams = [...new Set(allGames.map((game) => game.team))];
  for (const team of teams) {
    const teamSlug = team.toLowerCase().replace(/\s+/g, "");
    const teamDir = path.join(distDir, teamSlug);
    await fs.mkdir(teamDir, { recursive: true });
    await generateHtml(allGames, path.join(teamDir, "index.html"), team);
  }

  console.log(
    "\n✓ Done! Generated dist/index.html and individual team pages in dist/[team]/index.html.",
  );
}

// Run the script
if (require.main === module) {
  main().catch((error) => {
    console.error("Error:", error.message);
    process.exit(1);
  });
}
