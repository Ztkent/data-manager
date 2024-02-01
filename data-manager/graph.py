from collections import namedtuple
import networkx as nx
import matplotlib.pyplot as plt
import db 
import graph


def show_directed_graph(urls: db.LinkData):
    urls = [(link.referrer, link.url) for link in urls]
    G = nx.DiGraph()
    for referrer, url in urls:
        G.add_edge(referrer, url)

    nx.draw(G, with_labels=True)
    plt.show()

def show_spring_layout_graph(urls: db.LinkData):
    urls = [(link.referrer, link.url) for link in urls]
    G = nx.DiGraph()
    for referrer, url in urls:
        G.add_edge(referrer, url)
    pos = nx.spring_layout(G)
    nx.draw(G, pos, with_labels=True)
    plt.show()