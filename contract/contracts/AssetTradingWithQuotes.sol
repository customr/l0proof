// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "./OracleVerifier.sol";

/**
 * @title AssetTradingWithQuotes
 * @dev Simplified contract for demo purposes that verifies price quotes
 */
contract AssetTradingWithQuotes {
    // Reference to the oracle verifier
    OracleVerifier public oracleVerifier;

    event PriceVerified(address indexed user, string data);
    event PriceRejected(address indexed user, string reason);

    address public owner;

    modifier onlyOwner() {
        require(msg.sender == owner, "Not authorized");
        _;
    }

    constructor(address _oracleVerifier) {
        owner = msg.sender;
        oracleVerifier = OracleVerifier(_oracleVerifier);
    }

    /**
     * @dev Set a new oracle verifier address
     * @param _oracleVerifier The address of the new oracle verifier
     */
    function setOracleVerifier(address _oracleVerifier) external onlyOwner {
        oracleVerifier = OracleVerifier(_oracleVerifier);
    }

    function calculateHash(
        string calldata data,
        uint256 timestamp
    ) external pure returns (bytes32 message) {
        return keccak256(abi.encodePacked(data, timestamp));
    }

    /**
     * @dev Process a price verification with raw data and timestamp
     * @param data The raw data string
     * @param timestamp The timestamp when the data was created
     * @param signatures The signatures from the oracles
     */
    function processPrice(
        string calldata data,
        uint256 timestamp,
        bytes[] calldata signatures
    ) external {
        bool isValid = oracleVerifier.verify(data, signatures, timestamp);

        if (isValid) {
            emit PriceVerified(msg.sender, data);
        } else {
            emit PriceRejected(msg.sender, "Invalid price verification");
            revert("Invalid price verification");
        }
    }
}
