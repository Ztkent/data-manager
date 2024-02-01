from collections import namedtuple
import sqlite3
from sqlite3 import Error


def create_db_conn(db_file):
    conn = None
    try:
        conn = sqlite3.connect(db_file)
        print("Database connection successful")
    except Error as e:
        print(f"Error connecting to database: {e}")
        raise
    if conn is None:
        raise ValueError("Failed to create the database connection")
    return conn

LinkData = namedtuple('Link', ['id', 'url', 'referrer', 'last_visited_at', 'is_complete', 'is_blocked'])
def fetch_all_links(conn):
    cur = conn.cursor()
    links = [LinkData]
    try:
        cur.execute("SELECT * FROM visited")
        rows = cur.fetchall()
        for row in rows:
            link = LinkData(*row)
            links.append(link)
    except Error as e:
        print("Error executing query: ", e)
    return links

def print_all_links(conn):
    cur = conn.cursor()
    try:
        cur.execute("SELECT url FROM visited")
        rows = cur.fetchall()
        for row in rows:
            print(row)
    except Error as e:
        print("Error executing query: ", e)