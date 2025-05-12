const hre = require("hardhat");

async function main() {
  // Verifier contract address
  const verifierAddress = "0x1B198e1A81b40D4679F97ffF1a8390A4aB072D75";
  
  // Oracles to add
  const oracleAddresses = [
    "0x281a56D355eeD275a09Cad4BeaE9b43dA42A7D7b",
    "0xCE4Fb20eeE6269a9F4CFBBf82d8E4FB58E9aBC6B",
    "0x0B872b104A9E8D9c2687318742314d30Bad5Ff63"
  ];

  try {
    // Get the deployer account
    const [deployer] = await ethers.getSigners();
    console.log(`Using account: ${deployer.address}`);
    
    // Get contract instance
    const verifier = await ethers.getContractAt("OracleVerifier", verifierAddress);
    
    // Check if we're the owner
    const owner = await verifier.owner();
    if (owner.toLowerCase() !== deployer.address.toLowerCase()) {
      console.error("Error: Your account is not the owner of this contract");
      process.exit(1);
    }
    
    console.log(`Current oracle count: ${await verifier.oracleCount()}`);
    console.log(`Current threshold: ${await verifier.threshold()}`);
    
    // Add each oracle
    for (const oracleAddress of oracleAddresses) {
      // Check if oracle is already added
      const isAlreadyOracle = await verifier.trustedOracles(oracleAddress);
      if (isAlreadyOracle) {
        console.log(`Oracle ${oracleAddress} is already trusted`);
        continue;
      }
      
      // Add the oracle
      console.log(`Adding oracle ${oracleAddress}...`);
      const tx = await verifier.addOracle(oracleAddress);
      
      // Wait for transaction to be mined
      console.log(`Transaction hash: ${tx.hash}`);
      await tx.wait();
      console.log(`Transaction confirmed for ${oracleAddress}`);
      
      // Check if the oracle is now trusted
      const isTrusted = await verifier.trustedOracles(oracleAddress);
      console.log(`Oracle ${oracleAddress} is ${isTrusted ? 'now trusted' : 'not trusted'}`);
    }
    
    // Show updated state
    console.log(`Updated oracle count: ${await verifier.oracleCount()}`);
    console.log(`Updated threshold: ${await verifier.threshold()}`);
    
  } catch (error) {
    console.error("Error:", error.message);
    process.exit(1);
  }
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  }); 