import sqlite3
from sqlite3 import Error
import argparse

from db import create_db_conn, fetch_all_links
from graph import save_pyvis_link_graph, save_networkx_link_graph

def main():
    parser = argparse.ArgumentParser(description="Process and visualize data from a database.")
    parser.add_argument('--database', default='user/data-crawler/results.db', help='The path to the database')
    parser.add_argument('--output', default='user/network/network.html', help='The path to the output file')

    args = parser.parse_args()

    conn = create_db_conn(args.database)
    if conn is not None:
        with conn:
            links = fetch_all_links(conn)
            save_pyvis_link_graph(links, False, args.output)
            # save_networkx_link_graph(links, args.output)
    else:
        print(f"Failed to create database connection to {args.database}")

if __name__ == "__main__":
    main()