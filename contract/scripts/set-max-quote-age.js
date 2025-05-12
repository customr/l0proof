const hre = require("hardhat");

async function main() {
  // Get the contract address and max age from command line arguments
  const verifierAddress = "0x1B198e1A81b40D4679F97ffF1a8390A4aB072D75";
  const maxAgeInSeconds = "100";

  if (!verifierAddress || !maxAgeInSeconds) {
    console.error("Usage: npx hardhat run scripts/set-max-quote-age.js <verifier-address> <max-age-in-seconds>");
    process.exit(1);
  }

  if (isNaN(maxAgeInSeconds) || parseInt(maxAgeInSeconds) <= 0) {
    console.error("Max age must be a positive number in seconds");
    process.exit(1);
  }

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
    
    // Get current max age
    const currentMaxAge = await verifier.maxQuoteAge();
    console.log(`Current max quote age: ${currentMaxAge} seconds`);
    
    // Set new max age
    console.log(`Setting max quote age to ${maxAgeInSeconds} seconds...`);
    
    const tx = await verifier.setMaxQuoteAge(maxAgeInSeconds);
    
    // Wait for transaction to be mined
    console.log(`Transaction hash: ${tx.hash}`);
    await tx.wait();
    console.log("Transaction confirmed");
    
    // Verify the new max age
    const newMaxAge = await verifier.maxQuoteAge();
    console.log(`New max quote age: ${newMaxAge} seconds`);
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