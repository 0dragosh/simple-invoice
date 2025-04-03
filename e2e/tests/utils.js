const { faker } = require('@faker-js/faker');
const fs = require('fs');
const path = require('path');
const pdfParse = require('pdf-parse');

/**
 * Generate random business data with EUR and USD bank accounts
 */
function generateBusinessData() {
  return {
    name: `${faker.company.name()} ${faker.string.alphanumeric(4)}`,
    address: faker.location.streetAddress(),
    city: faker.location.city(),
    postalCode: faker.location.zipCode(),
    country: faker.location.country(),
    vatID: `${faker.string.alpha(2).toUpperCase()}${faker.number.int({min: 100000000, max: 999999999})}`,
    email: faker.internet.email(),
    bankName: faker.company.name() + ' Bank',
    iban: `${faker.string.alpha(2).toUpperCase()}${faker.string.alphanumeric(30)}`,
    bic: faker.string.alpha(8).toUpperCase(),
    bankNameUSD: faker.company.name() + ' USD Bank',
    ibanUSD: `${faker.string.alpha(2).toUpperCase()}${faker.string.alphanumeric(30)}`,
    bicUSD: faker.string.alpha(8).toUpperCase(),
    registrationNumber: faker.number.int({min: 10000000, max: 99999999}).toString(),
    website: faker.internet.url(),
    phone: faker.phone.number()
  };
}

/**
 * Generate random client data
 */
function generateClientData() {
  return {
    name: `${faker.company.name()} ${faker.string.alphanumeric(4)}`,
    address: faker.location.streetAddress(),
    city: faker.location.city(),
    postalCode: faker.location.zipCode(),
    country: faker.location.country(),
    vatID: `${faker.string.alpha(2).toUpperCase()}${faker.number.int({min: 100000000, max: 999999999})}`,
    email: faker.internet.email(),
    phone: faker.phone.number()
  };
}

/**
 * Generate invoice data
 */
function generateInvoiceData(businessName, clientName, currency = 'EUR', vatType = 'normal') {
  const baseItems = [
    {
      description: `${faker.commerce.productName()} - ${faker.word.words(3)}`,
      quantity: faker.number.int({min: 1, max: 5}),
      unitPrice: faker.number.int({min: 100, max: 1000})
    },
    {
      description: `${faker.commerce.productName()} - ${faker.word.words(3)}`,
      quantity: faker.number.int({min: 1, max: 5}),
      unitPrice: faker.number.int({min: 100, max: 1000})
    }
  ];
  
  // Determine VAT rate based on type
  let vatRate = 0;
  if (vatType === 'normal') {
    vatRate = 20; // Standard VAT rate
  } else if (vatType === 'reverse-charge') {
    vatRate = 0; // Reverse charge VAT is 0%
  }
  
  return {
    invoiceNumber: `INV-${faker.number.int({min: 10000, max: 99999})}`,
    issueDate: faker.date.recent().toISOString().split('T')[0],
    dueDate: faker.date.future().toISOString().split('T')[0],
    currency: currency,
    notes: vatType === 'reverse-charge' 
      ? 'Reverse charge VAT applies. VAT to be accounted for by the recipient.'
      : faker.lorem.sentence(),
    vatRate: vatRate,
    items: baseItems
  };
}

/**
 * Parse PDF content and check for expected values
 */
async function validatePDF(pdfPath, expectedValues) {
  if (!fs.existsSync(pdfPath)) {
    throw new Error(`PDF file not found at path: ${pdfPath}`);
  }
  
  const dataBuffer = fs.readFileSync(pdfPath);
  const pdfData = await pdfParse(dataBuffer);
  const content = pdfData.text;
  
  const validation = { success: true, errors: [] };
  
  for (const [key, value] of Object.entries(expectedValues)) {
    if (typeof value === 'string' && value.length > 0) {
      if (!content.includes(value)) {
        validation.success = false;
        validation.errors.push(`Expected value "${value}" for "${key}" not found in PDF`);
      }
    }
  }
  
  // Also check for unwanted characters
  const unwantedPatterns = [
    /undefined/,
    /null/,
    /NaN/,
    /\[object Object\]/,
    /Error:/
  ];
  
  for (const pattern of unwantedPatterns) {
    if (pattern.test(content)) {
      validation.success = false;
      validation.errors.push(`Unwanted pattern "${pattern}" found in PDF`);
    }
  }
  
  return validation;
}

module.exports = {
  generateBusinessData,
  generateClientData,
  generateInvoiceData,
  validatePDF
}; 