import sqlite3
from sqlite3 import Error

from db import create_db_conn, fetch_all_links
from graph import show_spring_layout_graph

def main():
    database = "crawl_results.db"
    conn = create_db_conn(database)
    if conn is not None:
        with conn:
            links = fetch_all_links(conn)
            show_spring_layout_graph(links)
    else:
        print("Failed to create database connection")


if __name__ == "__main__":
    main()