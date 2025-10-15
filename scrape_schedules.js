#!/usr/bin/env node
/**
 * Basketball Team Schedule Scraper
 * Scrapes multiple team schedules from tourneymachine.com and combines them into a single HTML page.
 */

const axios = require("axios");
const cheerio = require("cheerio");
const fs = require("fs").promises;
const path = require("path");

// Team URLs - Add more teams here
// Format: displayName: { url: "...", htmlName: "exact name as it appears in the HTML", color: "#RRGGBB" }
const TEAM_URLS = {
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
    color: "#2196F3",
  },
  "10U Red": {
    url: "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263e6b6d69f385c49&IDTeam=h202508032216206132b484a6720f345",
    htmlName: "Omaha Lightning Red 4th",
    color: "red",
  },
  "10U Black": {
    url: "https://tourneymachine.com/Public/Results/Team.aspx?IDTournament=h2025031418210726136d760ccca8e44&IDDivision=h20250314182107263934d14719c5d45&IDTeam=h202508032216205157e930ef2d5314d",
    htmlName: "Omaha Lightning Black 3rd",
    color: "black",
  },
  // "Display Name": { url: "URL", htmlName: "Exact HTML Name", color: "#RRGGBB" },
};

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
 * Generate HTML schedule page
 */
async function generateHtml(allGames, outputFile = "combined_schedule.html", filterTeam = null) {
  // Filter games if a specific team is requested
  const gamesToDisplay = filterTeam
    ? allGames.filter(game => game.team === filterTeam)
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

  // Get unique teams in the order they appear in TEAM_URLS
  const teamOrder = Object.keys(TEAM_URLS);
  const teams = [...new Set(allGames.map((game) => game.team))].sort((a, b) => {
    return teamOrder.indexOf(a) - teamOrder.indexOf(b);
  });

  // Create a map of team names to colors
  const teamColorMap = {};
  allGames.forEach(game => {
    teamColorMap[game.team] = game.color;
  });

  let html = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Lightning Combined Schedule</title>
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
            border-collapse: collapse;
            background-color: white;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        th {
            background-color: #fbcb44;
            color: white;
            padding: 12px;
            text-align: left;
        }
        td {
            padding: 10px;
            border-bottom: 1px solid #ddd;
        }
        tr:hover {
            background-color: #f5f5f5;
        }
        .team-badge {
            display: inline-block;
            padding: 4px 8px;
            background-color: #2196F3;
            color: white;
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
        @media (max-width: 768px) {
            table {
                font-size: 0.9em;
            }
            th, td {
                padding: 8px 4px;
            }
        }
    </style>
</head>
<body>
    <h1>⚡️ Lightning Combined Schedule</h1>
    <p style="text-align: center; color: #999; font-size: 0.75rem; margin: -10px 0 20px 0;">Last updated on ${new Date().toLocaleDateString("en-US", { month: "numeric", day: "numeric", year: "2-digit" })} at ${new Date().toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit", hour12: true })}</p>
    <div class="filter-buttons">
        <a href="${filterTeam ? '../' : './'}" class="filter-btn ${!filterTeam ? 'active' : ''}" style="${!filterTeam ? 'background-color: #fbcb44 !important; color: black !important;' : ''} text-decoration: none; display: inline-block;">All Teams</a>
`;

  // Add filter buttons for each team with their respective colors
  teams.forEach((team) => {
    const teamColor = teamColorMap[team];
    const textColor = teamColor === "#FFFFFF" || teamColor === "white" ? "black" : "white";
    const teamSlug = team.toLowerCase().replace(/\s+/g, '');
    const activeClass = filterTeam === team ? 'active' : '';
    const activeStyle = filterTeam === team ? `background-color: ${teamColor} !important; color: ${textColor} !important;` : '';
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
  sortedGames.forEach((game) => {
    // Format date to be more concise (e.g., "Sat, Oct 18, 2025")
    let displayDate = game.date || "TBD";
    try {
      const dateObj = new Date(game.date);
      if (!isNaN(dateObj.getTime())) {
        displayDate = dateObj.toLocaleDateString("en-US", {
          weekday: "short",
          month: "short",
          day: "numeric",
          year: "numeric",
        });
      }
    } catch (e) {
      // Keep original date if parsing fails
    }

    // Format jersey text
    let jerseyText = "TBD";
    if (game.homeAway === "Home") {
      jerseyText = "Home (Light)";
    } else if (game.homeAway === "Away") {
      jerseyText = "Away (Dark)";
    }

    // Get team color
    const teamColor = game.color;
    const textColor = teamColor === "#FFFFFF" ? "#000000" : "#FFFFFF";

    html += `            <tr class="game-row" data-team="${game.team}">
                <td><span class="team-badge" style="background-color: ${teamColor}; color: ${textColor};">${game.team}</span></td>
                <td>${displayDate}</td>
                <td>${game.time || "TBD"}</td>
                <td>${game.location || "TBD"}</td>
                <td>${jerseyText}</td>
                <td>${game.opponent || "TBD"}</td>
                <td>${game.score || "-"}</td>
            </tr>
`;
  });

  html += `        </tbody>
    </table>
</body>
</html>
`;

  await fs.writeFile(outputFile, html, "utf-8");
  console.log(`\nGenerated ${outputFile}`);
}

/**
 * Main function to scrape all teams and generate combined schedule
 */
async function main() {
  console.log("Starting schedule scraper...\n");

  const allGames = [];

  for (const [displayName, teamInfo] of Object.entries(TEAM_URLS)) {
    const games = await scrapeTeamSchedule(
      displayName,
      teamInfo.url,
      teamInfo.htmlName,
      teamInfo.color,
    );
    allGames.push(...games);
  }

  if (allGames.length === 0) {
    console.log("\nNo games found. Please check the URLs and try again.");
    process.exit(1);
  }

  console.log(`\nTotal games found: ${allGames.length}`);

  // Create dist directory
  const distDir = path.join(process.cwd(), 'dist');
  await fs.mkdir(distDir, { recursive: true });

  // Generate combined schedule as index.html in dist
  await generateHtml(allGames, path.join(distDir, "index.html"));

  // Generate individual team schedules in subfolders
  const teams = [...new Set(allGames.map(game => game.team))];
  for (const team of teams) {
    const teamSlug = team.toLowerCase().replace(/\s+/g, '');
    const teamDir = path.join(distDir, teamSlug);
    await fs.mkdir(teamDir, { recursive: true });
    await generateHtml(allGames, path.join(teamDir, "index.html"), team);
  }

  console.log("\n✓ Done! Generated dist/index.html and individual team pages in dist/[team]/index.html.");
}

// Run the script
if (require.main === module) {
  main().catch((error) => {
    console.error("Error:", error.message);
    process.exit(1);
  });
}
