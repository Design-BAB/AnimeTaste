AnimeTaste 🍥✨

Are you constantly bickering with your friends about who has the best anime taste? Or perhaps you just want a fun, meme-filled way to gauge your own preferences? Look no further! The Anime Taste Checker is a web application built with Go and HTMX that lets you pick your favorite anime and then delivers a verdict on your taste, complete with a healthy dose of meme-y judgment.

🚀 Features

    Anime Search: Easily find your favorite anime using the integrated search functionality, powered by a local SQLite database and the Jikan API (for extended search results).

    Dynamic Selection: Add anime to your personal list on the fly, thanks to the power of HTMX for seamless user experience.

    Taste Evaluation: Get a hilarious (and sometimes brutally honest) assessment of your anime preferences based on your selections.

    Meme-tastic Verdicts: Prepare for some lighthearted roasts or high praise, depending on your picks!

🛠️ Technologies Used

    Go: The backend logic, database interaction, and API handling are all powered by Go, known for its performance and concurrency.

    HTMX: This lightweight JavaScript library allows for dynamic HTML updates without writing a lot of custom JavaScript, making the user experience snappy and interactive.

    SQLite: A file-based database used to store anime information locally, ensuring quick access to frequently searched titles.

    Jikan API: When your local database doesn't have the anime you're looking for, the app reaches out to the Jikan API to fetch more data.
