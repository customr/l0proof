const hre = require("hardhat");

async function main() {
  console.log("Deploying contracts...");
  
  // Get deployer account
  const [deployer] = await ethers.getSigners();
  console.log(`Deploying contracts with the account: ${deployer.address}`);
  console.log(`Account balance: ${(await deployer.provider.getBalance(deployer.address)).toString()}`);
  
  // Deploy OracleVerifier
  const OracleVerifier = await hre.ethers.getContractFactory("OracleVerifier");
  const verifier = await OracleVerifier.deploy();
  await verifier.waitForDeployment();
  
  const verifierAddress = await verifier.getAddress();
  console.log(`OracleVerifier deployed to: ${verifierAddress}`);
  
  // Deploy AssetTradingWithQuotes
  const AssetTradingWithQuotes = await hre.ethers.getContractFactory("AssetTradingWithQuotes");
  const trading = await AssetTradingWithQuotes.deploy(verifierAddress);
  await trading.waitForDeployment();
  
  const tradingAddress = await trading.getAddress();
  console.log(`AssetTradingWithQuotes deployed to: ${tradingAddress}`);
  
  console.log("Deployment completed");

}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  }); 