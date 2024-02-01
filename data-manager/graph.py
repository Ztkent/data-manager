from pyvis.network import Network

import db 

def save_pyvis_link_graph(urls: db.LinkData):
    urls = [(link.referrer, link.url) for link in urls[1:]]
    G = Network()
    if G is None:
        raise ValueError("Failed to create Network object")
    
    for referrer, url in urls:
        if referrer == "STARTING_URL":
            continue
        G.add_node(referrer)
        G.add_node(url)
        G.add_edge(referrer, url)

    # Display the graph
    G.save_graph("network.html")