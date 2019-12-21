package policy
import input.attributes as req

default decision = false

decision {
    req.request.http.host == "localhost:10000"
    req.source.address.Address.SocketAddress.address == "127.0.0.1"
}
