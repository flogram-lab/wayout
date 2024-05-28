import GRPC
import WayoutLib

print("Hello, world!")

print("Begin gRPC test")

let group = PlatformSupport.makeEventLoopGroup(loopCount: 1)
defer {
  try? group.syncShutdownGracefully()
}

let channel = try GRPCChannelPool.with(
  target: .host("localhost", port: 8920),
  transportSecurity: .plaintext,
  eventLoopGroup: group
)
defer {
  try? channel.close().wait()
}

let flotg = WayoutLib.FlotgServiceAsyncClient(channel: channel)
await flotg.getChats(pbempty)

print("SUCCESS!");