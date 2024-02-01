import sqlite3
from sqlite3 import Error
import argparse

from db import create_db_conn, fetch_all_links
from graph import save_pyvis_link_graph

def main():
    parser = argparse.ArgumentParser(description="Process and visualize data from a database.")
    parser.add_argument('--database', default='results.db', help='The path to the database')

    args = parser.parse_args()

    conn = create_db_conn(args.database)
    if conn is not None:
        with conn:
            links = fetch_all_links(conn)
            save_pyvis_link_graph(links)
    else:
        print(f"Failed to create database connection to {args.database}")

if __name__ == "__main__":
    main()