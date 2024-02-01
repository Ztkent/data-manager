# Data Manager

A Python module that processes, classifies, and visualizes data.  
It integrates with the [Data-Crawler](https://github.com/Ztkent/data-crawler) project.

## Structure

The module is structured as follows:
- `data_manager.py`: This is the main script that manages the data processing. Any exported functionality is provided here.
- `db.py`: Handles database connections and operations.
- `graph.py`: Responsible for creating PyVis graphs.

## Usage
```bash
python data_manager.py --database path_to_your_database.db
python data_manager.py  # Uses 'results.db' as the default value
```