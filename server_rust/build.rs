fn main() -> Result<(), Box<dyn std::error::Error>> {
    // 添加 protobuf include 路径
    let protobuf_include = std::path::Path::new("/usr/local/opt/protobuf/include");
    
    let mut config = tonic_build::configure()
        .build_server(true)
        .build_client(false);
    
    // 如果系统有 protobuf 路径，添加它
    if protobuf_include.exists() {
        config = config.protoc_arg(format!("--proto_path={}", protobuf_include.display()));
        // 使用 prost_types 而不是自己编译
        config = config.extern_path(".google.protobuf", "::prost_types");
    }
    
    config.compile_protos(&["proto/monitor.proto"], &["proto"])?;
    
    println!("cargo:rerun-if-changed=proto/monitor.proto");
    Ok(())
}
