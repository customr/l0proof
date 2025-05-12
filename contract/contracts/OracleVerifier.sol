// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * @title OracleVerifier
 * @dev Contract for verifying offchain signed messages from trusted oracles
 */
contract OracleVerifier {
    mapping(address => bool) public trustedOracles;

    uint256 public oracleCount;

    uint256 public threshold;

    uint256 public maxQuoteAge = 300;

    event OracleAdded(address indexed oracle);
    event OracleRemoved(address indexed oracle);
    event ThresholdUpdated(uint256 newThreshold);
    event MaxQuoteAgeUpdated(uint256 newMaxAge);

    address public owner;

    modifier onlyOwner() {
        require(msg.sender == owner, "Not authorized");
        _;
    }

    constructor() {
        owner = msg.sender;
    }

    /**
     * @dev Add a trusted oracle address
     * @param oracle Address of the oracle to add
     */
    function addOracle(address oracle) external onlyOwner {
        require(!trustedOracles[oracle], "Oracle already added");
        trustedOracles[oracle] = true;
        oracleCount++;

        // Update threshold
        threshold = (oracleCount / 2) + 1;
        emit ThresholdUpdated(threshold);
        emit OracleAdded(oracle);
    }

    /**
     * @dev Remove a trusted oracle address
     * @param oracle Address of the oracle to remove
     */
    function removeOracle(address oracle) external onlyOwner {
        require(trustedOracles[oracle], "Oracle not found");
        trustedOracles[oracle] = false;
        oracleCount--;

        // Update threshold
        threshold = (oracleCount / 2) + 1;
        emit ThresholdUpdated(threshold);
        emit OracleRemoved(oracle);
    }

    /**
     * @dev Set the maximum age for quotes
     * @param _maxQuoteAge Maximum age in seconds
     */
    function setMaxQuoteAge(uint256 _maxQuoteAge) external onlyOwner {
        maxQuoteAge = _maxQuoteAge;
        emit MaxQuoteAgeUpdated(_maxQuoteAge);
    }

    /**
     * @dev Verify the signature for a message
     * @param messageHash The message hash that was signed
     * @param signature The signature in bytes
     * @return signer The address that signed the message
     */
    function recoverSigner(
        bytes32 messageHash,
        bytes memory signature
    ) public pure returns (address) {
        require(signature.length == 65, "Invalid signature length");

        bytes32 r;
        bytes32 s;
        uint8 v;

        assembly {
            r := mload(add(signature, 32))
            s := mload(add(signature, 64))
            v := byte(0, mload(add(signature, 96)))
        }

        bytes32 ethSignedMessageHash = keccak256(
            abi.encodePacked("\x19Ethereum Signed Message:\n32", messageHash)
        );

        return ecrecover(ethSignedMessageHash, v, r, s);
    }

    /**
     * @dev Verify a hash has enough valid signatures
     * @param data Message
     * @param signatures Array of signatures from oracles
     * @param timestamp The timestamp when the message was created
     * @return isValid Whether the message is valid
     */
    function verify(
        string calldata data,
        bytes[] calldata signatures,
        uint256 timestamp
    ) external view returns (bool isValid) {
        uint256 currentTime = block.timestamp;
        require(currentTime >= timestamp, "Timestamp is in the future");
        require(currentTime - timestamp <= maxQuoteAge, "Message is too old");

        require(signatures.length >= threshold, "Not enough signatures");

        address[] memory confirmedOracleList = new address[](signatures.length);
        uint256 confirmedCount = 0;

        for (uint256 i = 0; i < signatures.length; i++) {
            address signer = recoverSigner(
                keccak256(abi.encodePacked(data, timestamp)),
                signatures[i]
            );

            if (trustedOracles[signer]) {
                bool isDuplicate = false;
                for (uint256 j = 0; j < confirmedCount; j++) {
                    if (confirmedOracleList[j] == signer) {
                        isDuplicate = true;
                        break;
                    }
                }

                if (!isDuplicate) {
                    confirmedOracleList[confirmedCount] = signer;
                    confirmedCount++;

                    if (confirmedCount >= threshold) {
                        return true;
                    }
                }
            }
        }

        return false;
    }
}
